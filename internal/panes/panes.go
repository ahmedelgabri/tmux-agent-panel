// Package panes builds the picker's rows. Panes running Claude Code, Codex,
// or pi show the agent icon in the leading slot and an agent column
// (state glyph + task) instead of the
// command column, fed by the @agent_state/@agent_task pane options that the
// agents set via hooks/extensions; which agent a pane runs comes from the
// @agent_name option the hooks set, falling back to the pane's current
// command for panes that never reported one. Agent rows sort first (blocked,
// waiting, running, idle) above plain panes; all rows share one column
// layout — window, agent-or-command, status, path — so the grouping reads
// from the aligned columns rather than from header rows, which fzf's
// filtering would tear apart anyway; a blank spacer row sets the agent
// block apart. The
// `_shared` session is skipped: its windows are linked into the named
// sessions, so listing it would only duplicate rows.
package panes

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rivo/uniseg"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/state"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/tmux"
)

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
// (used by --preview and the preview label). Non-selectable rows (the
// spacer, the orphan warning) have an empty PaneID, making selecting them
// a no-op.
type Row struct {
	PaneID  string
	Addr    string
	Display string

	// Split busy rows so fzf can animate the glyph without reloading items.
	spinnerPrefix, spinnerSuffix string
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠇"}

// PickerFields selects a spinner frame from the immutable picker row fields.
func PickerFields(frame int) string {
	return fmt.Sprintf("{3}{%d}{%d}", 4+frame%len(spinnerFrames), 4+len(spinnerFrames))
}

// Options carries the ambient inputs so BuildRows stays pure and testable.
type Options struct {
	AgentsOnly  bool
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
	_, named := agents.ByName(p.Agent)
	_, commanded := agents.ForCommand(p.Command)
	return p.State != "" && !named && !commanded
}

// BuildRows renders panes into one sorted, column-aligned table.
func BuildRows(lines []string, o Options) []Row {
	type entry struct {
		pane Pane
		meta agents.Agent
	}
	var entries []entry
	hasAgents := false
	winWidth, cmdWidth, taskWidth, orphans := 0, 0, 0, 0

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
		// The name column holds the agent name on agent rows and the pane
		// command otherwise; it shares one width so the table stays aligned.
		name := p.Command
		if isAgent {
			hasAgents = true
			name = meta.Name
			// The task lives in the pane title for TaskFromTitle agents;
			// the leading status glyph is dropped here.
			if p.Task == "" && meta.TaskFromTitle {
				p.Task = stripFirstWord(p.Title)
			}
			if p.State == "" {
				p.State = fallbackState(meta, p.Title)
			}
			if r := []rune(p.Task); len(r) > state.TaskMaxLength {
				p.Task = string(r[:state.TaskMaxLength-1]) + "…"
			}
			if w := uniseg.StringWidth(p.Task); w > taskWidth {
				taskWidth = w
			}
		}
		if w := uniseg.StringWidth(p.Window); w > winWidth {
			winWidth = w
		}
		if w := uniseg.StringWidth(name); w > cmdWidth {
			cmdWidth = w
		}
		entries = append(entries, entry{pane: p, meta: meta})
	}

	// Plain rows leave the agent rows' status slot — glyph, its trailing
	// space, and the padded task — blank so the path column stays aligned.
	plainGap := "  "
	if hasAgents {
		plainGap = "  " + pad("", taskWidth+2) + "  "
	}
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
		if e.meta.Name != "" {
			icon = e.meta.Color + e.meta.Icon + ansi.Reset
		}
		lead := mark + " " + icon + "  " + pad(p.Window, winWidth) + "  "

		if e.meta.Name != "" {
			prefix := lead + e.meta.Color + pad(e.meta.Name, cmdWidth) + ansi.Reset + "  "
			suffix := " " + pad(p.Task, taskWidth) + "  " + ansi.Gray + p.Path + ansi.Reset
			status := state.ByName(p.State)
			row := Row{PaneID: p.ID, Addr: p.Addr, Display: prefix + status.Color + status.Glyph + ansi.Reset + suffix}
			if p.State == "running" {
				row.spinnerPrefix = prefix + status.Color
				row.spinnerSuffix = ansi.Reset + suffix
			}
			agentRows = append(agentRows, agentRow{row: row, rank: status.Rank})
		} else {
			display := lead + ansi.Cyan + pad(p.Command, cmdWidth) + ansi.Reset + plainGap + ansi.Gray + p.Path + ansi.Reset
			plainRows = append(plainRows, Row{PaneID: p.ID, Addr: p.Addr, Display: display})
		}
	}

	sort.SliceStable(agentRows, func(a, b int) bool { return agentRows[a].rank < agentRows[b].rank })

	rows := make([]Row, 0, len(entries)+2)
	for _, a := range agentRows {
		rows = append(rows, a.row)
	}
	// A blank spacer sets the agent block apart when both groups are
	// present; its empty PaneID makes Enter on it a no-op, and any query
	// filters it out.
	if len(agentRows) > 0 && len(plainRows) > 0 {
		rows = append(rows, Row{})
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

// Render emits standalone list rows: pane_id, address, display, tab-separated.
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

// Keep rows independent of the clock so unchanged panes do not need reloads.
func listOptions(agentsOnly bool, home string) Options {
	current := os.Getenv(CurrentPaneEnv)
	if current == "" {
		current, _ = tmux.Output("display-message", "-p", "#{pane_id}")
	}
	return Options{
		AgentsOnly:  agentsOnly,
		CurrentPane: current,
		Home:        home,
	}
}

// SnapshotEnv lets reload children consume the exact rows the poller compared,
// rather than a second tmux sample that could change between reads.
const SnapshotEnv = "TAP_PICKER_SNAPSHOT"

type Snapshot struct {
	All, Agents string
	HasAgents   bool
	Animated    bool
}

// Write publishes both views together so toggling cannot read a partial update.
func (s Snapshot) Write(path string) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	defer os.Remove(tmp)
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// List uses the popup's snapshot when available; standalone callers query tmux.
func List(agentsOnly bool, home string) (string, error) {
	if path := os.Getenv(SnapshotEnv); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return "", err
		}
		if agentsOnly {
			return snapshot.Agents, nil
		}
		return snapshot.All, nil
	}
	out, err := listOutput()
	if err != nil {
		return "", err
	}
	return Render(BuildRows(strings.Split(out, "\n"), listOptions(agentsOnly, home))), nil
}

// ListViews renders both views from one tmux sample, ignoring any cached snapshot.
func ListViews(home string) (Snapshot, error) {
	out, err := listOutput()
	if err != nil {
		return Snapshot{}, err
	}
	lines := strings.Split(out, "\n")
	o := listOptions(true, home)
	agents := BuildRows(lines, o)
	agentText, animated := renderPicker(agents)
	o.AgentsOnly = false
	allText, _ := renderPicker(BuildRows(lines, o))
	return Snapshot{
		All:       allText,
		Agents:    agentText,
		HasAgents: selectable(agents),
		Animated:  animated,
	}, nil
}

// Each snapshot carries all spinner frames. change-with-nth changes only the
// display projection, avoiding the input blocking caused by tracked reloads.
func renderPicker(rows []Row) (string, bool) {
	var b strings.Builder
	animated := false
	for _, r := range rows {
		prefix := r.Display
		if r.spinnerPrefix != "" {
			prefix = r.spinnerPrefix
			animated = true
		}
		b.WriteString(r.PaneID + "\t" + r.Addr + "\t" + prefix)
		for _, glyph := range spinnerFrames {
			b.WriteByte('\t')
			if r.spinnerPrefix != "" {
				b.WriteString(glyph)
			}
		}
		b.WriteString("\t" + r.spinnerSuffix + "\n")
	}
	return b.String(), animated
}

// selectable reports whether any row targets a real pane — non-selectable
// rows (spacer, orphan warning) leave PaneID empty.
func selectable(rows []Row) bool {
	for _, r := range rows {
		if r.PaneID != "" {
			return true
		}
	}
	return false
}

// pad counts terminal cells, not runes: CJK and emoji content in tasks or
// window names occupies two cells per rune, and rune-based padding would
// misalign the columns after it.
func pad(s string, width int) string {
	if n := width - uniseg.StringWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
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
	return strings.IndexFunc(s, func(r rune) bool { return r < ' ' || r > '~' }) >= 0
}

func stripFirstWord(s string) string {
	if _, rest, ok := strings.Cut(s, " "); ok {
		return rest
	}
	return s
}
