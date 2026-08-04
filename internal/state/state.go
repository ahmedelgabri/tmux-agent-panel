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

	"github.com/ahmedelgabri/tmux-agent-panel/internal/tmux"
)

// TaskMaxLength caps @agent_task so rows stay scannable.
const TaskMaxLength = 60

var validStates = map[string]bool{
	"running": true,
	"idle":    true,
	"waiting": true,
	"blocked": true,
}

// Set records the given state on $TMUX_PANE and returns what it resolved to
// ("clear", or a concrete state — notification payloads resolve to
// blocked/waiting). Outside tmux it is a no-op returning "": hooks fire for
// agents running anywhere, but only tmux panes can display state. "clear"
// unsets both options. With titleStdin, the hook JSON's .prompt becomes
// @agent_task.
func Set(st string, titleStdin bool, stdin io.Reader) (string, error) {
	pane := os.Getenv("TMUX_PANE")
	if !tmux.InsideTmux() || pane == "" {
		return "", nil
	}

	if st == "clear" {
		// Unset fails when the option was never set; that is fine.
		_ = tmux.Run("set-option", "-pu", "-t", pane, "@agent_state")
		_ = tmux.Run("set-option", "-pu", "-t", pane, "@agent_task")
		return "clear", nil
	}

	var payload []byte
	if st == "notification" || titleStdin {
		payload, _ = io.ReadAll(stdin)
	}
	if st == "notification" {
		st = RouteNotification(payload)
	}
	if !validStates[st] {
		return "", fmt.Errorf("invalid state %q (want running|idle|waiting|blocked|notification|clear)", st)
	}

	if err := tmux.Run("set-option", "-p", "-t", pane, "@agent_state", st); err != nil {
		return "", err
	}

	if titleStdin {
		if task := TaskFromPrompt(payload); task != "" {
			if err := tmux.Run("set-option", "-p", "-t", pane, "@agent_task", task); err != nil {
				return "", err
			}
		}
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

// TaskFromPrompt extracts .prompt from hook JSON and normalizes it to a
// single line of at most TaskMaxLength runes.
func TaskFromPrompt(payload []byte) string {
	var p struct {
		Prompt string `json:"prompt"`
	}
	_ = json.Unmarshal(payload, &p)
	line := p.Prompt
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
