package panes

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/osc7501"
)

func TestParseRex(t *testing.T) {
	out := `[
		{"block_id":"block:a","session":"main","window":"code","window_index":1,"pane_index":2,"command":".claude-wrapped","path":"/x","pane_title":"✳ fix","state":"done","app":"claude-code","children":[{"id":"sub1","state":"working","title":"Explore"}]},
		{"block_id":"block:b","session":"main","window":"build","window_index":2,"pane_index":1,"command":"brew","path":"/x","state":"working","app":"brew","msg":"upgrading"},
		{"block_id":"block:c","session":"main","window":"pi","window_index":3,"pane_index":1,"command":"pi","state":"blocked","kind":"question","app":"pi","msg":"session"},
		{"block_id":"block:d","session":"main","window":"sh","window_index":4,"pane_index":1,"command":"zsh","state":"working"},
		{"block_id":"block:e","session":"main","window":"ci","window_index":5,"pane_index":1,"command":"make","state":"working","title":"Deploy"},
		{"block_id":"block:picker","session":"main","window":"code","window_index":1,"pane_index":3,"command":"tap"}
	]`
	got, err := parseRex(out, "block:picker")
	if err != nil {
		t.Fatal(err)
	}
	want := []Pane{
		{
			ID: "block:a", Addr: "main:1.2", Window: "code", Command: ".claude-wrapped", Path: "/x", State: "idle", Title: "✳ fix", App: "claude-code",
			Children: []osc7501.Record{{ID: "sub1", State: "working", Title: "Explore"}},
		},
		{ID: "block:b", Addr: "main:2.1", Window: "build", Command: "brew", Path: "/x", State: "running", Task: "upgrading", App: "brew"},
		{ID: "block:c", Addr: "main:3.1", Window: "pi", Command: "pi", State: "waiting", Task: "session", App: "pi"},
		// A record without a title or an app names nothing, so it is not agent state.
		{ID: "block:d", Addr: "main:4.1", Window: "sh", Command: "zsh"},
		{ID: "block:e", Addr: "main:5.1", Window: "ci", Command: "make", State: "running", AppTitle: "Deploy"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if ps, err := parseRex("{}", ""); err != nil || len(ps) != 0 {
		t.Fatalf("no panes: %v, %v", ps, err)
	}
}

func TestRecordAgentRows(t *testing.T) {
	ps := []Pane{
		{ID: "a", Window: "w", Command: "node", State: "running", App: "opencode"},
		{ID: "b", Window: "w", Command: "brew", State: "idle", Task: "upgraded", App: "brew"},
		{ID: "c", Window: "w", Command: "cargo", State: "running", App: "cargo", AppTitle: "Release build"},
	}
	rows := buildRows(ps, Options{AgentsOnly: true})
	if len(rows) != 3 {
		t.Fatalf("every program that reports status is an agent row, got %d rows", len(rows))
	}
	byID := map[string]string{}
	for _, r := range rows {
		byID[r.PaneID] = r.Display
	}
	if d := byID["a"]; !strings.Contains(d, "▣") || !strings.Contains(d, "opencode") {
		t.Errorf("a known app should use its agent's icon and name: %q", d)
	}
	if d := byID["b"]; !strings.Contains(d, "●") || !strings.Contains(d, "brew") || !strings.Contains(d, "upgraded") {
		t.Errorf("an unknown app should be a generic agent named after it: %q", d)
	}
	if d := byID["c"]; !strings.Contains(d, "Release build") || strings.Contains(d, "cargo") {
		t.Errorf("a record's title should name its agent before its app: %q", d)
	}
}

func TestSubagentRows(t *testing.T) {
	ps := []Pane{
		{ID: "idle", Addr: "main:1.1", Window: "w", State: "idle", App: "pi"},
		{ID: "busy", Addr: "main:2.1", Window: "w", State: "blocked", App: "claude-code", Children: []osc7501.Record{
			{ID: "a", State: "working", Title: "Explore picker"},
			{ID: "b", State: "blocked", Kind: "question", Msg: "Pick a key"},
		}},
	}
	rows := buildRows(ps, Options{AgentsOnly: true})
	if len(rows) != 4 {
		t.Fatalf("want an agent row per pane plus a row per subagent, got %d", len(rows))
	}
	// Subagents follow their agent even when it sorts first, and share its
	// pane so Enter and the preview go there.
	for i, want := range []string{"busy", "busy", "busy", "idle"} {
		if rows[i].PaneID != want || (rows[i].PaneID == "busy" && rows[i].Addr != "main:2.1") {
			t.Fatalf("row %d: got pane %q (%s), want %q", i, rows[i].PaneID, rows[i].Addr, want)
		}
	}
	// The picker tracks the cursor by pane ID and key, so rows that share a
	// pane need distinct keys.
	seen := map[[2]string]bool{}
	for _, r := range rows {
		id := [2]string{r.PaneID, r.Key}
		if seen[id] {
			t.Fatalf("rows share the identity %q", id)
		}
		seen[id] = true
	}
	if d := rows[1].Display; !strings.Contains(d, "├─") || !strings.Contains(d, "Explore picker") || rows[1].spinnerPrefix == "" {
		t.Errorf("a running subagent should branch off its agent and animate: %q", d)
	}
	if d := rows[2].Display; !strings.Contains(d, "└─") || !strings.Contains(d, "?") || !strings.Contains(d, "Pick a key") {
		t.Errorf("the last subagent should close the branch and fall back to its message: %q", d)
	}
}
