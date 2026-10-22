package panes

import (
	"reflect"
	"testing"
)

func TestParseRex(t *testing.T) {
	out := `[
		{"block_id":"block:a","session":"main","window":"code","window_index":1,"pane_index":2,"command":".claude-wrapped","path":"/x","title":"✳ fix","state":"done","app":"claude-code"},
		{"block_id":"block:b","session":"main","window":"build","window_index":2,"pane_index":1,"command":"brew","path":"/x","state":"working","app":"brew","msg":"upgrading"},
		{"block_id":"block:c","session":"main","window":"pi","window_index":3,"pane_index":1,"command":"pi","state":"blocked","kind":"question","app":"pi","msg":"session"},
		{"block_id":"block:picker","session":"main","window":"code","window_index":1,"pane_index":3,"command":"tap"}
	]`
	got, err := parseRex(out, "block:picker")
	if err != nil {
		t.Fatal(err)
	}
	want := []Pane{
		{ID: "block:a", Addr: "main:1.2", Window: "code", Command: ".claude-wrapped", Path: "/x", State: "idle", Title: "✳ fix", Agent: "claude"},
		// A non-agent's record is not agent state, or it would be an orphan.
		{ID: "block:b", Addr: "main:2.1", Window: "build", Command: "brew", Path: "/x"},
		{ID: "block:c", Addr: "main:3.1", Window: "pi", Command: "pi", State: "waiting", Task: "session", Agent: "pi"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if ps, err := parseRex("{}", ""); err != nil || len(ps) != 0 {
		t.Fatalf("no panes: %v, %v", ps, err)
	}
}
