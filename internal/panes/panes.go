// Package panes builds the picker's rows. Panes running Claude Code, Codex,
// or pi show an agent column (icon + state glyph + task) instead of the
// command column, fed by the @agent_state/@agent_task pane options that the
// agents set via hooks/extensions; which agent a pane runs is derived from
// its current command. Agent rows sort first (blocked, waiting, running,
// idle), separated from plain panes by divider rows. The `_shared` session
// is skipped: its windows are linked into the named sessions, so listing it
// would only duplicate rows.
package panes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/state"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/tmux"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠇"}

// listFormat matches the field order Pane and BuildRows expect.
const listFormat = "#{pane_id}\t#{session_name}:#{window_index}.#{pane_index}\t#{window_name}\t#{pane_current_command}\t#{pane_current_path}\t#{" + state.StateOption + "}\t#{" + state.TaskOption + "}\t#{pane_title}"

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
}

// Row is one picker entry. Key exists only to order the list; PaneID and
// Addr ride along as hidden fzf fields (used by --preview and the preview
// label). Divider rows have an empty PaneID, making selecting them a no-op.
type Row struct {
	Key     string
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
	if len(f) < 8 {
		return Pane{}, false
	}
	return Pane{
		ID: f[0], Addr: f[1], Window: f[2], Command: f[3],
		Path: f[4], State: f[5], Task: f[6], Title: f[7],
	}, true
}

// AgentFor maps a pane's current command to an agent name, or "" for plain
// panes.
func AgentFor(command string) string {
	if a, ok := agents.ForCommand(command); ok {
		return a.Name
	}
	return ""
}

// BuildRows renders panes into sorted picker rows, dividers included.
func BuildRows(lines []string, o Options) []Row {
	type entry struct {
		pane    Pane
		meta    agents.Agent
		isAgent bool
	}
	var entries []entry
	winWidth, cmdWidth, agentCount := 0, 0, 0

	for _, line := range lines {
		p, ok := ParsePane(line)
		if !ok {
			continue
		}
		meta, isAgent := agents.ForCommand(p.Command)
		if o.AgentsOnly && !isAgent {
			continue
		}
		if o.Home != "" && strings.HasPrefix(p.Path, o.Home) {
			p.Path = "~" + strings.TrimPrefix(p.Path, o.Home)
		}
		if w := len([]rune(p.Window)); w > winWidth {
			winWidth = w
		}
		if isAgent {
			agentCount++
		} else {
			if w := len([]rune(p.Command)); w > cmdWidth {
				cmdWidth = w
			}
		}
		entries = append(entries, entry{p, meta, isAgent})
	}

	spinner := spinnerFrames[o.Frame%len(spinnerFrames)]
	var rows []Row
	for i, e := range entries {
		p := e.pane
		// Blue dot marks the pane the picker was opened from; ❐ is a
		// regular session pane, ⧉ a persistent popup.
		mark := " "
		if p.ID == o.CurrentPane {
			mark = ansi.BoldBlue + "●" + ansi.Reset
		}
		paneType := "❐"
		if strings.HasPrefix(p.Addr, "popup_") {
			paneType = "⧉"
		}
		lead := mark + " " + ansi.Gray + paneType + ansi.Reset + "  " + pad(p.Window, winWidth) + "  "

		var key, display string
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
			key = fmt.Sprintf("1%d%06d", state.ByName(st).Rank, i)
			display = lead + e.meta.Icon + "  " + stateGlyph(st, spinner) + " " + title + "  " + ansi.Gray + p.Path + ansi.Reset
		} else {
			key = fmt.Sprintf("3%07d", i)
			display = lead + ansi.Cyan + pad(p.Command, cmdWidth) + ansi.Reset + "  " + ansi.Gray + p.Path + ansi.Reset
		}
		rows = append(rows, Row{Key: key, PaneID: p.ID, Addr: p.Addr, Display: display})
	}

	// Dividers only make sense when both groups are present.
	if !o.AgentsOnly && agentCount > 0 && agentCount < len(entries) {
		rows = append(
			rows,
			Row{Key: "0", Display: ansi.Gray + "──── agents ────" + ansi.Reset},
			Row{Key: "2", Display: ansi.Gray + "──── panes ─────" + ansi.Reset},
		)
	}

	sort.Slice(rows, func(a, b int) bool { return rows[a].Key < rows[b].Key })
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

// List shells out to tmux and renders the current rows. The spinner frame
// comes from a 200ms clock so consecutive reloads animate it.
func List(agentsOnly bool, home string) (string, error) {
	out, err := tmux.Output("list-panes", "-a", "-f", "#{!=:#{session_name},_shared}", "-F", listFormat)
	if err != nil {
		return "", err
	}
	current, _ := tmux.Output("display-message", "-p", "#{pane_id}")
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
