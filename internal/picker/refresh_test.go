package picker

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/panes"
)

func TestRefreshLoop(t *testing.T) {
	// t.TempDir paths can exceed macOS's Unix socket path limit.
	dir, err := os.MkdirTemp("/tmp", "tap-refresh-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "fzf.sock")
	listener, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	type status struct {
		Query   string `json:"query"`
		Reading bool   `json:"reading"`
	}
	var mu sync.Mutex
	var current status
	rows := panes.Snapshot{All: "initial rows", Agents: "initial agent rows", HasAgents: true}
	gets := make(chan status, 100)
	posts := make(chan string, 100)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			mu.Lock()
			snapshot := current
			mu.Unlock()
			json.NewEncoder(w).Encode(snapshot)
			gets <- snapshot
			return
		}
		body, _ := io.ReadAll(r.Body)
		posts <- string(body)
	})}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	cache := filepath.Join(dir, "rows.json")
	if err := rows.Write(cache); err != nil {
		t.Fatal(err)
	}
	initial := rows
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		refreshLoop(sock, "/bin/tap", cache, initial, func() (panes.Snapshot, error) {
			mu.Lock()
			defer mu.Unlock()
			return rows, nil
		}, stop)
	}()
	t.Cleanup(func() {
		close(stop)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("refresh loop did not stop")
		}
	})
	wantPost := func(want string) {
		t.Helper()
		select {
		case got := <-posts:
			if got != want {
				t.Fatalf("action = %q, want %q", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
	wantGet := func(want status) {
		t.Helper()
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for {
			select {
			case got := <-gets:
				if got == want {
					return
				}
			case <-timer.C:
				t.Fatalf("timed out waiting for status %+v", want)
			}
		}
	}
	noPosts := func() {
		t.Helper()
		select {
		case action := <-posts:
			t.Fatalf("unexpected action while reading or typing: %s", action)
		default:
		}
	}

	// Keep preview output live without rebuilding identical rows, even at startup.
	wantPost("refresh-preview")
	wantPost("refresh-preview")
	mu.Lock()
	rows = panes.Snapshot{All: "changed rows", Agents: "changed agent rows", HasAgents: true}
	current.Reading = true
	mu.Unlock()
	for range 3 {
		wantGet(status{Reading: true})
	}
	noPosts()

	// Changes stay pending while input arrives, even beyond the quiet delay.
	for _, query := range []string{"s", "se", "sec", "seco", "secon", "second"} {
		mu.Lock()
		current = status{Query: query}
		mu.Unlock()
		wantGet(status{Query: query})
		noPosts()
	}
	wantPost(reloadAction("/bin/tap"))
	wantPost("refresh-preview")
	t.Setenv(panes.SnapshotEnv, cache)
	for agentsOnly, want := range map[bool]string{false: "changed rows", true: "changed agent rows"} {
		got, err := panes.List(agentsOnly, "")
		if err != nil || got != want {
			t.Errorf("reload snapshot for agents=%v: %q, %v", agentsOnly, got, err)
		}
	}
}
