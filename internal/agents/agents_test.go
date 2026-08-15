package agents

import "testing"

func TestForCommand(t *testing.T) {
	cases := map[string]string{
		"claude":          "claude",
		".claude-wrapped": "claude",
		".codex-wrapped":  "codex",
		"pi":              "pi",
		".pi-wrapped":     "pi",
		"nvim":            "",
		"claudette":       "",
		"zsh":             "",
	}
	for in, want := range cases {
		a, ok := ForCommand(in)
		got := ""
		if ok {
			got = a.Name
		}
		if got != want {
			t.Errorf("ForCommand(%q) = %q, want %q", in, got, want)
		}
	}
}
