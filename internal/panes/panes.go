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

	"github.com/ahmedelgabri/tmux-agent-panel/internal/tmux"
)

const (
	reset   = "\033[0m"
	gray    = "\033[90m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[1;34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"

	titleMax = 60
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠇"}

// listFormat matches the field order Pane and BuildRows expect.
const listFormat = "#{pane_id}\t#{session_name}:#{window_index}.#{pane_index}\t#{window_name}\t#{pane_current_command}\t#{pane_current_path}\t#{@agent_state}\t#{@agent_task}\t#{pane_title}"

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
// panes. Home Manager wraps binaries, so agents can show up as e.g.
// ".claude-wrapped" — strip the wrapper to detect them.
func AgentFor(command string) string {
	c := strings.TrimPrefix(command, ".")
	c = strings.TrimSuffix(c, "-wrapped")
	switch c {
	case "claude", "codex", "pi":
		return c
	}
	return ""
}

// BuildRows renders panes into sorted picker rows, dividers included.
func BuildRows(lines []string, o Options) []Row {
	type entry struct {
		pane  Pane
		agent string
	}
	var entries []entry
	winWidth, cmdWidth, agents := 0, 0, 0

	for _, line := range lines {
		p, ok := ParsePane(line)
		if !ok {
			continue
		}
		agent := AgentFor(p.Command)
		if o.AgentsOnly && agent == "" {
			continue
		}
		if o.Home != "" && strings.HasPrefix(p.Path, o.Home) {
			p.Path = "~" + strings.TrimPrefix(p.Path, o.Home)
		}
		if w := len([]rune(p.Window)); w > winWidth {
			winWidth = w
		}
		if agent == "" {
			if w := len([]rune(p.Command)); w > cmdWidth {
				cmdWidth = w
			}
		} else {
			agents++
		}
		entries = append(entries, entry{p, agent})
	}

	spinner := spinnerFrames[o.Frame%len(spinnerFrames)]
	var rows []Row
	for i, e := range entries {
		p := e.pane
		// Blue dot marks the pane the picker was opened from; ❐ is a
		// regular session pane, ⧉ a persistent popup.
		mark := " "
		if p.ID == o.CurrentPane {
			mark = blue + "●" + reset
		}
		paneType := "❐"
		if strings.HasPrefix(p.Addr, "popup_") {
			paneType = "⧉"
		}
		lead := mark + " " + gray + paneType + reset + "  " + pad(p.Window, winWidth) + "  "

		var key, display string
		if e.agent != "" {
			st := p.State
			title := p.Task
			// Claude publishes a live task summary in the pane title
			// (behind a status glyph) — better than the raw prompt, so
			// state hooks don't store a task for it and the glyph is
			// dropped here instead.
			if title == "" && e.agent == "claude" {
				title = stripFirstWord(p.Title)
			}
			if st == "" {
				st = fallbackState(e.agent, p.Title)
			}
			if r := []rune(title); len(r) > titleMax {
				title = string(r[:titleMax-1]) + "…"
			}
			key = fmt.Sprintf("1%d%06d", stateRank(st), i)
			display = lead + agentIcon(e.agent) + "  " + stateGlyph(st, spinner) + " " + title + "  " + gray + p.Path + reset
		} else {
			key = fmt.Sprintf("3%07d", i)
			display = lead + cyan + pad(p.Command, cmdWidth) + reset + "  " + gray + p.Path + reset
		}
		rows = append(rows, Row{Key: key, PaneID: p.ID, Addr: p.Addr, Display: display})
	}

	// Dividers only make sense when both groups are present.
	if !o.AgentsOnly && agents > 0 && agents < len(entries) {
		rows = append(
			rows,
			Row{Key: "0", Display: gray + "──── agents ────" + reset},
			Row{Key: "2", Display: gray + "──── panes ─────" + reset},
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

func stateRank(st string) int {
	switch st {
	case "blocked":
		return 0
	case "waiting":
		return 1
	case "running":
		return 2
	}
	return 3
}

// Per-agent icon in a distinct ANSI slot: Claude yellow (closest named slot
// to its orange), Codex cyan, pi magenta.
func agentIcon(agent string) string {
	switch agent {
	case "claude":
		return yellow + "✳" + reset
	case "codex":
		return cyan + "⌬" + reset
	}
	return magenta + "π" + reset
}

func stateGlyph(st, spinner string) string {
	switch st {
	case "running":
		return green + spinner + reset
	case "blocked":
		return red + "▲" + reset
	case "waiting":
		return yellow + "?" + reset
	}
	return gray + "◌" + reset
}

// Claude Code prefixes its pane title with a spinner glyph while working
// and ✳ when waiting; use that until hooks set the option.
func fallbackState(agent, title string) string {
	if agent != "claude" {
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
