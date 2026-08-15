// Package panes builds the picker's rows. Panes running Claude Code, Codex,
// or pi show the agent icon in the leading slot and an agent column
// (state glyph + task) instead of the
// command column, fed by the @agent_state/@agent_task pane options that the
// agents set via hooks/extensions; which agent a pane runs comes from the
// @agent_name option the hooks set, falling back to the pane's current
// command for panes that never reported one. Agent rows sort first (blocked,
// waiting, running, idle), separated from plain panes by divider rows. The
// `_shared` session is skipped: its windows are linked into the named
// sessions, so listing it would only duplicate rows.
package panes

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/state"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/tmux"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠇"}

// CurrentPaneEnv carries the pane the picker was opened from. The picker
// resolves it once before starting fzf — it cannot change while the popup
// is open — and reload children inherit it, saving a tmux round trip on
// every refresh tick.
const CurrentPaneEnv = "TAP_CURRENT_PANE"

// listFormat matches the field order Pane and BuildRows expect.
const listFormat = "#{pane_id}\t#{session_name}:#{window_index}.#{pane_index}\t#{window_name}\t#{pane_current_command}\t#{pane_current_path}\t#{" + state.StateOption + "}\t#{" + state.TaskOption + "}\t#{pane_title}\t#{" + state.AgentOption + "}"

// Pane is one line of `tmux list-panes` output.
type Pane struct {
	ID      string
	Addr    string // session:window.pane
	Window  string
	Command string
	Path    string
	State   string // @agent_state, may be empty
	Task    string // @agent_task, may be empty
	Title   string // pane_title
	Agent   string // @agent_name, may be empty
}

// Row is one picker entry. PaneID and Addr ride along as hidden fzf fields
// (used by --preview and the preview label). Divider rows have an empty
// PaneID, making selecting them a no-op.
type Row struct {
	PaneID  string
	Addr    string
	Display string
}

// Options carries the ambient inputs so BuildRows stays pure and testable.
type Options struct {
	AgentsOnly  bool
	Frame       int    // spinner frame index
	CurrentPane string // pane the picker was opened from
	Home        string // for ~ shortening
}

// ParsePane splits one tab-separated list-panes line.
func ParsePane(line string) (Pane, bool) {
	f := strings.Split(line, "\t")
	if len(f) < 9 {
		return Pane{}, false
	}
	return Pane{
		ID: f[0], Addr: f[1], Window: f[2], Command: f[3],
		Path: f[4], State: f[5], Task: f[6], Title: f[7], Agent: f[8],
	}, true
}

// Orphaned reports whether a pane records agent state without a resolvable
// agent identity: hooks that don't pass --agent, or options gone stale
// after an unclean agent exit. The picker warns about such panes and
// doctor explains them; neither guesses an agent.
func Orphaned(p Pane) bool {
	if p.State == "" {
		return false
	}
	if _, ok := agents.ByName(p.Agent); ok {
		return false
	}
	_, ok := agents.ForCommand(p.Command)
	return !ok
}

// BuildRows renders panes into sorted picker rows, dividers included.
func BuildRows(lines []string, o Options) []Row {
	type entry struct {
		pane    Pane
		meta    agents.Agent
		isAgent bool
	}
	var entries []entry
	winWidth, cmdWidth, orphans := 0, 0, 0

	for _, line := range lines {
		p, ok := ParsePane(line)
		if !ok {
			continue
		}
		// Counted before the agents-only filter: an orphan is by definition
		// not an agent row, but the warning must show in both views.
		if Orphaned(p) {
			orphans++
		}
		// The hook-reported @agent_name wins: pane_current_command is
		// unreliable on some systems (wrappers, generic interpreters).
		meta, isAgent := agents.ByName(p.Agent)
		if !isAgent {
			meta, isAgent = agents.ForCommand(p.Command)
		}
		if o.AgentsOnly && !isAgent {
			continue
		}
		if o.Home != "" && strings.HasPrefix(p.Path, o.Home) {
			p.Path = "~" + strings.TrimPrefix(p.Path, o.Home)
		}
		if w := len([]rune(p.Window)); w > winWidth {
			winWidth = w
		}
		if !isAgent {
			if w := len([]rune(p.Command)); w > cmdWidth {
				cmdWidth = w
			}
		}
		entries = append(entries, entry{p, meta, isAgent})
	}

	spinner := spinnerFrames[o.Frame%len(spinnerFrames)]
	// Agent rows sort by state rank (blocked first) but keep their
	// original order within a state; plain rows keep list-panes order.
	type agentRow struct {
		row  Row
		rank int
	}
	var agentRows []agentRow
	var plainRows []Row
	for _, e := range entries {
		p := e.pane
		// Blue dot marks the pane the picker was opened from.
		mark := " "
		if p.ID == o.CurrentPane {
			mark = ansi.BoldBlue + "●" + ansi.Reset
		}
		// The leading icon is the pane type — ❐ regular session, ⧉
		// persistent popup — except on agent rows, where which agent runs
		// there is the more useful fact.
		icon := ansi.Gray + "❐" + ansi.Reset
		if strings.HasPrefix(p.Addr, "popup_") {
			icon = ansi.Gray + "⧉" + ansi.Reset
		}
		if e.isAgent {
			icon = e.meta.Icon
		}
		lead := mark + " " + icon + "  " + pad(p.Window, winWidth) + "  "

		if e.isAgent {
			st := p.State
			title := p.Task
			// The task lives in the pane title for TaskFromTitle agents;
			// the leading status glyph is dropped here.
			if title == "" && e.meta.TaskFromTitle {
				title = stripFirstWord(p.Title)
			}
			if st == "" {
				st = fallbackState(e.meta, p.Title)
			}
			if r := []rune(title); len(r) > state.TaskMaxLength {
				title = string(r[:state.TaskMaxLength-1]) + "…"
			}
			display := lead + stateGlyph(st, spinner) + " " + title + "  " + ansi.Gray + p.Path + ansi.Reset
			agentRows = append(agentRows, agentRow{
				row:  Row{PaneID: p.ID, Addr: p.Addr, Display: display},
				rank: state.ByName(st).Rank,
			})
		} else {
			display := lead + ansi.Cyan + pad(p.Command, cmdWidth) + ansi.Reset + "  " + ansi.Gray + p.Path + ansi.Reset
			plainRows = append(plainRows, Row{PaneID: p.ID, Addr: p.Addr, Display: display})
		}
	}

	sort.SliceStable(agentRows, func(a, b int) bool { return agentRows[a].rank < agentRows[b].rank })

	// Dividers only make sense when both groups are present.
	divided := len(agentRows) > 0 && len(plainRows) > 0
	rows := make([]Row, 0, len(entries)+2)
	if divided {
		rows = append(rows, Row{Display: ansi.Gray + "──── agents ────" + ansi.Reset})
	}
	for _, a := range agentRows {
		rows = append(rows, a.row)
	}
	if divided {
		rows = append(rows, Row{Display: ansi.Gray + "──── panes ─────" + ansi.Reset})
	}
	rows = append(rows, plainRows...)
	// Orphaned panes are never guessed into agent rows (a stale option
	// after an unclean exit would render a ghost agent); a non-selectable
	// warning row points at doctor, which explains both causes.
	if orphans > 0 {
		noun := "pane reports"
		if orphans > 1 {
			noun = "panes report"
		}
		rows = append(rows, Row{Display: ansi.Yellow + "⚠" + ansi.Reset + ansi.Gray +
			fmt.Sprintf(" %d %s agent state without an agent — run `tap doctor`", orphans, noun) + ansi.Reset})
	}
	return rows
}

// Render emits rows in the picker's wire format: pane_id, address, display,
// tab-separated. fzf hides the first two via --with-nth=3...
func Render(rows []Row) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r.PaneID + "\t" + r.Addr + "\t" + r.Display + "\n")
	}
	return b.String()
}

func listOutput() (string, error) {
	return tmux.Output("list-panes", "-a", "-f", "#{!=:#{session_name},_shared}", "-F", listFormat)
}

// Fetch lists panes tmux-wide in parsed form.
func Fetch() ([]Pane, error) {
	out, err := listOutput()
	if err != nil {
		return nil, err
	}
	var ps []Pane
	for _, l := range strings.Split(out, "\n") {
		if p, ok := ParsePane(l); ok {
			ps = append(ps, p)
		}
	}
	return ps, nil
}

// List shells out to tmux and renders the current rows. The spinner frame
// comes from a 200ms clock so consecutive reloads animate it.
func List(agentsOnly bool, home string) (string, error) {
	out, err := listOutput()
	if err != nil {
		return "", err
	}
	current := os.Getenv(CurrentPaneEnv)
	if current == "" {
		current, _ = tmux.Output("display-message", "-p", "#{pane_id}")
	}
	o := Options{
		AgentsOnly:  agentsOnly,
		Frame:       int(time.Now().UnixMicro() / 200000),
		CurrentPane: current,
		Home:        home,
	}
	return Render(BuildRows(strings.Split(out, "\n"), o)), nil
}

func pad(s string, width int) string {
	if n := width - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func stateGlyph(st, spinner string) string {
	d := state.ByName(st)
	glyph := d.Glyph
	// running is the one animated state; its glyph is the current
	// spinner frame rather than a fixed rune.
	if glyph == "" {
		glyph = spinner
	}
	return d.Color + glyph + ansi.Reset
}

// TaskFromTitle agents prefix their pane title with a spinner glyph while
// working and ✳ when waiting; use that until hooks set the option.
func fallbackState(a agents.Agent, title string) string {
	if !a.TaskFromTitle {
		return "idle"
	}
	first, _, _ := strings.Cut(title, " ")
	if first != "✳" && containsNonASCII(first) {
		return "running"
	}
	return "idle"
}

func containsNonASCII(s string) bool {
	for _, r := range s {
		if r < ' ' || r > '~' {
			return true
		}
	}
	return false
}

func stripFirstWord(s string) string {
	if _, rest, ok := strings.Cut(s, " "); ok {
		return rest
	}
	return s
}
