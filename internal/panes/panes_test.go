package panes

import (
	"strings"
	"testing"
)

func line(fields ...string) string {
	return strings.Join(fields, "\t")
}

var fixture = []string{
	line("%1", "main:1.1", "editor", "nvim", "/Users/x/code", "", "", "nvim"),
	line("%2", "main:1.2", "agent", ".claude-wrapped", "/Users/x/code", "running", "fix the bug", "✳ fixing"),
	line("%3", "work:2.1", "shell", "zsh", "/Users/x", "", "", "zsh"),
	line("%4", "popup_dotfil_a3f2:1.1", "popup", "codex", "/Users/x/.dotfiles", "blocked", "review", "codex"),
	line("%5", "main:3.1", "pi", "pi", "/Users/x", "idle", "", "pi"),
}

func TestAgentFor(t *testing.T) {
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
		if got := AgentFor(in); got != want {
			t.Errorf("AgentFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildRowsOrdering(t *testing.T) {
	rows := BuildRows(fixture, Options{Home: "/Users/x"})
	if len(rows) != 7 {
		t.Fatalf("got %d rows, want 7 (5 panes + 2 dividers)", len(rows))
	}
	// Agents divider, then blocked codex, running claude, idle pi, panes
	// divider, then plain panes in original order.
	wantIDs := []string{"", "%4", "%2", "%5", "", "%1", "%3"}
	for i, want := range wantIDs {
		if rows[i].PaneID != want {
			t.Errorf("row %d: pane %q, want %q", i, rows[i].PaneID, want)
		}
	}
}

func TestBuildRowsAgentsOnly(t *testing.T) {
	rows := BuildRows(fixture, Options{AgentsOnly: true, Home: "/Users/x"})
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	for _, r := range rows {
		if r.PaneID == "" {
			t.Errorf("agents-only view must not contain dividers")
		}
	}
}

func TestBuildRowsNoDividersWhenHomogeneous(t *testing.T) {
	plain := []string{fixture[0], fixture[2]}
	if rows := BuildRows(plain, Options{}); len(rows) != 2 {
		t.Errorf("plain-only: got %d rows, want 2 (no dividers)", len(rows))
	}
	agents := []string{fixture[1], fixture[3]}
	if rows := BuildRows(agents, Options{}); len(rows) != 2 {
		t.Errorf("agents-only content: got %d rows, want 2 (no dividers)", len(rows))
	}
}

func TestBuildRowsDisplay(t *testing.T) {
	rows := BuildRows(fixture, Options{Home: "/Users/x", CurrentPane: "%3"})
	byID := map[string]Row{}
	for _, r := range rows {
		byID[r.PaneID] = r
	}

	if !strings.Contains(byID["%3"].Display, "●") {
		t.Errorf("current pane should carry the ● mark: %q", byID["%3"].Display)
	}
	if strings.Contains(byID["%1"].Display, "●") {
		t.Errorf("non-current pane should not carry the ● mark")
	}
	// Agent rows lead with the agent icon, not the pane-type icon.
	if !strings.Contains(byID["%4"].Display, "⌬") || strings.Contains(byID["%4"].Display, "⧉") {
		t.Errorf("agent pane should lead with its agent icon: %q", byID["%4"].Display)
	}
	if !strings.Contains(byID["%2"].Display, "✳") {
		t.Errorf("claude pane should lead with ✳: %q", byID["%2"].Display)
	}
	if !strings.Contains(byID["%1"].Display, "❐") {
		t.Errorf("session pane should use the ❐ icon")
	}
	if !strings.Contains(byID["%1"].Display, "~/code") {
		t.Errorf("path should be ~-shortened: %q", byID["%1"].Display)
	}
	if !strings.Contains(byID["%2"].Display, "fix the bug") {
		t.Errorf("agent task should be shown: %q", byID["%2"].Display)
	}
	if !strings.Contains(byID["%4"].Display, "▲") {
		t.Errorf("blocked agent should show ▲: %q", byID["%4"].Display)
	}
}

func TestClaudeTitleFallback(t *testing.T) {
	// No @agent_task: Claude's pane title (minus the leading glyph) is the
	// task, and a non-✳ glyph means running.
	l := line("%9", "main:1.1", "w", "claude", "/", "", "", "⠹ compiling the thing")
	rows := BuildRows([]string{l}, Options{})
	if !strings.Contains(rows[0].Display, "compiling the thing") {
		t.Errorf("title fallback missing: %q", rows[0].Display)
	}
	if strings.Contains(rows[0].Display, "◌") {
		t.Errorf("spinner title should imply running, not idle: %q", rows[0].Display)
	}

	l = line("%9", "main:1.1", "w", "claude", "/", "", "", "✳ waiting around")
	rows = BuildRows([]string{l}, Options{})
	if !strings.Contains(rows[0].Display, "◌") {
		t.Errorf("✳ title should imply idle: %q", rows[0].Display)
	}
}

func TestTitleTruncation(t *testing.T) {
	long := strings.Repeat("x", 80)
	l := line("%9", "main:1.1", "w", "claude", "/", "running", long, "t")
	rows := BuildRows([]string{l}, Options{})
	if !strings.Contains(rows[0].Display, "…") {
		t.Errorf("long task should be truncated with …")
	}
	if strings.Contains(rows[0].Display, long) {
		t.Errorf("task must not exceed the cap")
	}
}

func TestRenderWireFormat(t *testing.T) {
	rows := BuildRows(fixture, Options{})
	out := Render(rows)
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if len(strings.SplitN(l, "\t", 3)) != 3 {
			t.Errorf("row is not 3 tab-separated fields: %q", l)
		}
	}
}
