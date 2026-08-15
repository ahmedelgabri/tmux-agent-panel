package agents

import "testing"

func TestParseVersion(t *testing.T) {
	cases := map[string]struct {
		v  [3]int
		ok bool
	}{
		"2.1.226 (Claude Code)": {[3]int{2, 1, 226}, true},
		"2.1.100\n":             {[3]int{2, 1, 100}, true},
		"":                      {[3]int{}, false},
		"Claude Code 2.1.226":   {[3]int{}, false},
		"2.1":                   {[3]int{}, false},
		"2.1.x":                 {[3]int{}, false},
	}
	for in, want := range cases {
		v, ok := parseVersion(in)
		if ok != want.ok || v != want.v {
			t.Errorf("parseVersion(%q) = %v, %v; want %v, %v", in, v, ok, want.v, want.ok)
		}
	}
}

func TestVersionLess(t *testing.T) {
	min := [3]int{2, 1, 101}
	cases := map[[3]int]bool{
		{2, 1, 100}: true,
		{2, 0, 999}: true,
		{1, 9, 999}: true,
		{2, 1, 101}: false,
		{2, 1, 226}: false,
		{3, 0, 0}:   false,
	}
	for v, want := range cases {
		if got := versionLess(v, min); got != want {
			t.Errorf("versionLess(%v, %v) = %v, want %v", v, min, got, want)
		}
	}
}

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
