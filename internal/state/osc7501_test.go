package state

import "testing"

func TestProgramStatusReport(t *testing.T) {
	cases := []struct {
		st, agent, task, want string
	}{
		{"running", "claude", "fix the bug", "\x1b]7501;state=working:id=pane-12:app=claude-code:msg=Zml4IHRoZSBidWc=\x1b\\"},
		{"blocked", "codex", "", "\x1b]7501;state=blocked:kind=permission:id=pane-12:app=codex\x1b\\"},
		{"waiting", "pi", "", "\x1b]7501;state=blocked:kind=question:id=pane-12:app=pi\x1b\\"},
		{"idle", "", "", "\x1b]7501;state=idle:id=pane-12\x1b\\"},
		{"clear", "claude", "task", "\x1b]7501;state=clear:id=pane-12\x1b\\"},
		// Control characters would void the whole report, so they go.
		{"idle", "", "a\x1bb\u0085", "\x1b]7501;state=idle:id=pane-12:msg=YWI=\x1b\\"},
		{"bogus", "", "", ""},
	}
	for _, c := range cases {
		if got := ProgramStatusReport(c.st, "%12", c.agent, c.task); got != c.want {
			t.Errorf("ProgramStatusReport(%q, %q, %q) = %q, want %q", c.st, c.agent, c.task, got, c.want)
		}
	}
}

func TestPassthrough(t *testing.T) {
	got := passthrough("\x1b]7501;state=idle\x1b\\")
	want := "\x1bPtmux;\x1b\x1b]7501;state=idle\x1b\x1b\\\x1b\\"
	if got != want {
		t.Errorf("passthrough = %q, want %q", got, want)
	}
}
