package panes

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRex(t *testing.T) {
	out := `[
		{"block_id":"block:a","session":"main","window":"code","window_index":1,"pane_index":2,"command":".claude-wrapped","path":"/x","pane_title":"✳ fix","state":"done","app":"claude-code"},
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
		{ID: "block:a", Addr: "main:1.2", Window: "code", Command: ".claude-wrapped", Path: "/x", State: "idle", Title: "✳ fix", App: "claude-code"},
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
