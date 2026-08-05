// Package state records coding-agent activity on the enclosing tmux pane so
// the picker can render agent rows. It is called from agent hooks (Claude
// Code settings.json, Codex hooks.json); hooks run as children of the agent,
// so $TMUX_PANE points at the right pane. Which agent it is isn't recorded —
// the picker derives that from the pane's current command.
package state

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/tmux"
)

// TaskMaxLength caps @agent_task so rows stay scannable.
const TaskMaxLength = 60

// Pane-scoped tmux user options tap owns. The picker reads what Set (and
// the pi extension) writes.
const (
	StateOption = "@agent_state"
	TaskOption  = "@agent_task"
)

// Desc describes one agent state: how it validates, sorts, and renders.
// Everything that varies per state lives here so adding one is a
// single-line change.
type Desc struct {
	Name  string
	Rank  int    // picker sort order within the agents section
	Glyph string // running has none: the picker animates a spinner instead
	Color string
}

// States is the canonical vocabulary, in rank order — blocked surfaces
// first because it needs the user.
var States = []Desc{
	{Name: "blocked", Rank: 0, Glyph: "▲", Color: ansi.Red},
	{Name: "waiting", Rank: 1, Glyph: "?", Color: ansi.Yellow},
	{Name: "running", Rank: 2, Glyph: "", Color: ansi.Green},
	{Name: "idle", Rank: 3, Glyph: "◌", Color: ansi.Gray},
}

// ByName returns the descriptor for a state, falling back to idle so
// unknown or stale values degrade to the least alarming rendering.
func ByName(name string) Desc {
	for _, d := range States {
		if d.Name == name {
			return d
		}
	}
	return States[len(States)-1]
}

// Names lists the state names in rank order.
func Names() []string {
	names := make([]string, len(States))
	for i, d := range States {
		names[i] = d.Name
	}
	return names
}

func valid(name string) bool {
	for _, d := range States {
		if d.Name == name {
			return true
		}
	}
	return false
}

// Set records the given state on $TMUX_PANE and returns what it resolved to
// ("clear", or a concrete state — notification payloads resolve to
// blocked/waiting). Outside tmux it is a no-op returning "": hooks fire for
// agents running anywhere, but only tmux panes can display state. "clear"
// unsets both options. The task comes from title (normalized free text,
// for programmatic callers like the pi extension) or, with titleStdin,
// from the hook JSON's .prompt on stdin.
func Set(st string, titleStdin bool, title string, stdin io.Reader) (string, error) {
	pane := os.Getenv("TMUX_PANE")
	if !tmux.InsideTmux() || pane == "" {
		return "", nil
	}

	if st == "clear" {
		// One tmux invocation for both options; a `;` argument separates
		// tmux commands, and the second runs even if the first fails
		// because an option was never set.
		_ = tmux.Run("set-option", "-pu", "-t", pane, StateOption, ";",
			"set-option", "-pu", "-t", pane, TaskOption)
		return "clear", nil
	}

	var payload []byte
	if st == "notification" || titleStdin {
		payload, _ = io.ReadAll(stdin)
	}
	if st == "notification" {
		st = RouteNotification(payload)
	}
	if !valid(st) {
		return "", fmt.Errorf("invalid state %q (want %s|notification|clear)", st, strings.Join(Names(), "|"))
	}

	task := NormalizeTask(title)
	if task == "" && titleStdin {
		task = TaskFromPrompt(payload)
	}
	args := []string{"set-option", "-p", "-t", pane, StateOption, st}
	if task != "" {
		args = append(args, ";", "set-option", "-p", "-t", pane, TaskOption, task)
	}
	if err := tmux.Run(args...); err != nil {
		return "", err
	}
	return st, nil
}

// RouteNotification maps a Notification hook payload to a state: permission
// requests block the agent, everything else (waiting for input, questions)
// is waiting. The match is on English message text; if the wording changes
// it degrades to waiting, never to a wrong blocked.
func RouteNotification(payload []byte) string {
	var msg struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(payload, &msg)
	if strings.Contains(strings.ToLower(msg.Message), "permission") {
		return "blocked"
	}
	return "waiting"
}

// TaskFromPrompt extracts .prompt from hook JSON and normalizes it.
func TaskFromPrompt(payload []byte) string {
	var p struct {
		Prompt string `json:"prompt"`
	}
	_ = json.Unmarshal(payload, &p)
	return NormalizeTask(p.Prompt)
}

// NormalizeTask reduces free text to a single line of at most
// TaskMaxLength runes, fit for a picker row.
func NormalizeTask(text string) string {
	line := text
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	line = strings.ReplaceAll(line, "\t", " ")
	runes := []rune(line)
	if len(runes) > TaskMaxLength {
		line = string(runes[:TaskMaxLength])
	}
	return line
}
