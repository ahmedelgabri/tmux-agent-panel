package picker

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/panes"
)

// BenchmarkRefresh compares cached reloads, in-process polling, and standalone
// listing. Every tmux command targets a private server, including cleanup.
func BenchmarkRefresh(b *testing.B) {
	if _, err := exec.LookPath("tmux"); err != nil {
		b.Skip("tmux not available")
	}
	// Keep Unix socket paths below macOS's length limit.
	dir, err := os.MkdirTemp("/tmp", "tap-bench-")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "tmux.sock")
	tmx := func(args ...string) {
		b.Helper()
		cmd := exec.Command("tmux", append([]string{"-S", socket, "-f", "/dev/null"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("private tmux: %v: %s", err, out)
		}
	}
	b.Setenv("TMUX", socket+",0,0")
	b.Setenv("TMUX_PANE", "%0")
	b.Setenv(panes.CurrentPaneEnv, "%0")
	b.Setenv("FZF_PROMPT", PromptAll)
	b.Setenv(panes.SnapshotEnv, "")
	tmx("new-session", "-d", "-s", "bench", "sleep 300")
	b.Cleanup(func() { _ = exec.Command("tmux", "-S", socket, "kill-server").Run() })
	tmx("set-option", "-p", "-t", "%0", "@agent_name", "codex")
	tmx("set-option", "-p", "-t", "%0", "@agent_state", "running")
	binary := filepath.Join(dir, "tap")
	if out, err := exec.Command("go", "build", "-o", binary, "../../cmd/tap").CombinedOutput(); err != nil {
		b.Fatalf("build: %v: %s", err, out)
	}
	home, _ := os.UserHomeDir()
	b.Run("shell-tap-tmux", func(b *testing.B) {
		for b.Loop() {
			if out, err := exec.Command("sh", "-c", shellQuote(binary)+" __list").CombinedOutput(); err != nil {
				b.Fatalf("reload: %v: %s", err, out)
			}
		}
	})
	b.Run("in-process-poll", func(b *testing.B) {
		for b.Loop() {
			if _, err := panes.ListViews(home); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("cached-reload", func(b *testing.B) {
		snapshot, err := panes.ListViews(home)
		if err != nil {
			b.Fatal(err)
		}
		cache := filepath.Join(dir, "rows.json")
		if err := snapshot.Write(cache); err != nil {
			b.Fatal(err)
		}
		b.Setenv(panes.SnapshotEnv, cache)
		for b.Loop() {
			if out, err := exec.Command("sh", "-c", shellQuote(binary)+" __list").CombinedOutput(); err != nil {
				b.Fatalf("cached reload: %v: %s", err, out)
			}
		}
	})
}
