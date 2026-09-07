package panes

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rivo/uniseg"
)

func line(fields ...string) string {
	return strings.Join(fields, "\t")
}

var fixture = []string{
	line("%1", "main:1.1", "editor", "nvim", "/Users/x/code", "", "", "nvim", ""),
	line("%2", "main:1.2", "agent", ".claude-wrapped", "/Users/x/code", "running", "fix the bug", "✳ fixing", ""),
	line("%3", "work:2.1", "shell", "zsh", "/Users/x", "", "", "zsh", ""),
	line("%4", "popup_dotfil_a3f2:1.1", "popup", "codex", "/Users/x/.dotfiles", "blocked", "review", "codex", ""),
	line("%5", "main:3.1", "pi", "pi", "/Users/x", "idle", "", "pi", ""),
}

func TestCachedList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rows.json")
	t.Setenv(SnapshotEnv, path)
	t.Setenv("PATH", t.TempDir())
	for _, snapshot := range []Snapshot{
		{All: "all\t'{}$(command)\n", Agents: "\x1b[32magent\x1b[0m\n", HasAgents: true},
		{All: "updated all", Agents: "updated agents", HasAgents: true},
	} {
		if err := snapshot.Write(path); err != nil {
			t.Fatal(err)
		}
		for agentsOnly, want := range map[bool]string{false: snapshot.All, true: snapshot.Agents} {
			got, err := List(agentsOnly, "")
			if err != nil || got != want {
				t.Fatalf("cached agents=%v: %q, %v", agentsOnly, got, err)
			}
		}
	}
	if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := List(false, ""); err == nil {
		t.Fatal("invalid cache must not silently fall back to live tmux")
	}
}

func TestRowsDoNotChangeWithClock(t *testing.T) {
	t.Setenv(CurrentPaneEnv, "%2")
	synctest.Test(t, func(t *testing.T) {
		before := Render(BuildRows(fixture, listOptions(false, "/Users/x")))
		time.Sleep(time.Second)
		after := Render(BuildRows(fixture, listOptions(false, "/Users/x")))
		if after != before {
			t.Fatal("clock-only changes must not trigger list reloads")
		}
	})
}

func TestOrphaned(t *testing.T) {
	cases := []struct {
		pane Pane
		want bool
	}{
		{Pane{Command: "zsh", State: "running"}, true},
		{Pane{Command: "node", State: "blocked", Agent: "bogus"}, true},
		{Pane{Command: "zsh"}, false},
		{Pane{Command: "claude", State: "running"}, false},
		{Pane{Command: "node", State: "running", Agent: "claude"}, false},
	}
	for _, c := range cases {
		if got := Orphaned(c.pane); got != c.want {
			t.Errorf("Orphaned(%+v) = %v, want %v", c.pane, got, c.want)
		}
	}
}

func TestOrphanWarningRow(t *testing.T) {
	orphan := line("%9", "main:1.1", "w", "zsh", "/", "running", "", "zsh", "")
	rows := BuildRows([]string{fixture[0], orphan}, Options{})
	last := rows[len(rows)-1]
	if last.PaneID != "" || !strings.Contains(last.Display, "tap doctor") {
		t.Errorf("expected non-selectable warning row, got %+v", last)
	}
	// The orphaned pane itself stays a plain row, not a guessed agent.
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (2 plain + warning)", len(rows))
	}

	rows = BuildRows([]string{orphan}, Options{AgentsOnly: true})
	if len(rows) != 1 || !strings.Contains(rows[0].Display, "tap doctor") {
		t.Errorf("agents-only view should still warn: %+v", rows)
	}

	for _, r := range BuildRows(fixture, Options{}) {
		if strings.Contains(r.Display, "⚠") {
			t.Errorf("no warning should show without orphans: %q", r.Display)
		}
	}
}

func TestAgentNameOverride(t *testing.T) {
	// @agent_name marks a pane whose command misreports (here: node) as an
	// agent row.
	l := line("%9", "main:1.1", "w", "node", "/", "running", "porting the picker", "t", "claude")
	rows := BuildRows([]string{l}, Options{})
	if !strings.Contains(rows[0].Display, "✳") {
		t.Errorf("@agent_name should classify the pane as claude: %q", rows[0].Display)
	}
	if !strings.Contains(rows[0].Display, "porting the picker") {
		t.Errorf("agent task should be shown: %q", rows[0].Display)
	}

	// An unrecognized @agent_name falls back to command detection.
	l = line("%9", "main:1.1", "w", "codex", "/", "running", "", "t", "bogus")
	rows = BuildRows([]string{l}, Options{})
	if !strings.Contains(rows[0].Display, "⌬") {
		t.Errorf("unknown @agent_name should fall back to the command: %q", rows[0].Display)
	}

	l = line("%9", "main:1.1", "w", "zsh", "/", "", "", "zsh", "bogus")
	rows = BuildRows([]string{l}, Options{AgentsOnly: true})
	if len(rows) != 0 {
		t.Errorf("unknown agent and command should stay a plain pane")
	}
}

func TestBuildRowsOrdering(t *testing.T) {
	rows := BuildRows(fixture, Options{Home: "/Users/x"})
	if len(rows) != 6 {
		t.Fatalf("got %d rows, want 6 (5 panes + spacer)", len(rows))
	}
	// Blocked codex, running claude, idle pi, a blank spacer, then plain
	// panes in original order.
	wantIDs := []string{"%4", "%2", "%5", "", "%1", "%3"}
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
			t.Errorf("agents-only view must not contain non-selectable rows")
		}
	}
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestBuildRowsColumnAlignment(t *testing.T) {
	rows := BuildRows(fixture, Options{Home: "/Users/x"})
	byID := map[string]Row{}
	for _, r := range rows {
		byID[r.PaneID] = r
	}
	agent := ansiRe.ReplaceAllString(byID["%2"].Display, "")
	plain := ansiRe.ReplaceAllString(byID["%1"].Display, "")
	// The name column shows the agent name, not the raw command.
	if !strings.Contains(agent, "claude") || strings.Contains(agent, ".claude-wrapped") {
		t.Errorf("agent row should show the agent name in the command column: %q", agent)
	}
	// The path column lines up across agent and plain rows. Offsets are
	// counted in terminal cells, matching what the picker renders.
	assertPathsAligned(t, agent, plain)
}

func assertPathsAligned(t *testing.T, a, b string) {
	t.Helper()
	aCol := uniseg.StringWidth(a[:strings.Index(a, "~/")])
	bCol := uniseg.StringWidth(b[:strings.Index(b, "~/")])
	if aCol != bCol {
		t.Errorf("path columns misaligned (%d vs %d):\n%q\n%q", aCol, bCol, a, b)
	}
}

func TestBuildRowsWideRuneAlignment(t *testing.T) {
	// CJK and emoji render wider than their rune count suggests, and
	// grapheme clusters (variation selectors: ❤️, flags: 🇪🇬 🇳🇱) even
	// measure wrong under per-rune width tables — only uniseg, which the
	// embedded fzf also renders with, gets all of them right.
	wide := []string{
		line("%1", "main:1.1", "编辑器", "nvim", "/Users/x/code", "", "", "nvim", ""),
		line("%2", "main:1.2", "agent", "claude", "/Users/x/code", "running", "修复 bug ❤️ 🇪🇬 🇳🇱", "t", ""),
	}
	rows := BuildRows(wide, Options{Home: "/Users/x"})
	byID := map[string]Row{}
	for _, r := range rows {
		byID[r.PaneID] = r
	}
	agent := ansiRe.ReplaceAllString(byID["%2"].Display, "")
	plain := ansiRe.ReplaceAllString(byID["%1"].Display, "")
	assertPathsAligned(t, agent, plain)
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
	l := line("%9", "main:1.1", "w", "claude", "/", "", "", "⠹ compiling the thing", "")
	rows := BuildRows([]string{l}, Options{})
	if !strings.Contains(rows[0].Display, "compiling the thing") {
		t.Errorf("title fallback missing: %q", rows[0].Display)
	}
	if strings.Contains(rows[0].Display, "◌") {
		t.Errorf("spinner title should imply running, not idle: %q", rows[0].Display)
	}

	l = line("%9", "main:1.1", "w", "claude", "/", "", "", "✳ waiting around", "")
	rows = BuildRows([]string{l}, Options{})
	if !strings.Contains(rows[0].Display, "◌") {
		t.Errorf("✳ title should imply idle: %q", rows[0].Display)
	}
}

func TestTitleTruncation(t *testing.T) {
	long := strings.Repeat("x", 80)
	l := line("%9", "main:1.1", "w", "claude", "/", "running", long, "t", "")
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
