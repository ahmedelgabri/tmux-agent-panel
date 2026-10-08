package state

import (
	"encoding/base64"
	"os"
	"strings"
	"syscall"
	"unicode"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
)

// ProgramStatusOption opts a pane (or, through tmux's option inheritance,
// a window, session, or the whole server) into mirroring agent state to the
// outer terminal via the Program Status Protocol (OSC 7501). It is opt-in
// because the sequence only gets through tmux with allow-passthrough on.
const ProgramStatusOption = "@tap_osc7501"

// programStatusQuery is appended to Set's tmux invocation, so mirroring
// costs no extra tmux spawn. It runs after the set-option commands, so the
// task and agent it reads are the ones just recorded. The task goes last
// because only it can contain the separator.
const programStatusQuery = "#{" + ProgramStatusOption + "}|#{pane_tty}|#{" + AgentOption + "}|#{" + TaskOption + "}"

// mirrorProgramStatus writes the report for st to the pane's tty when the
// pane opted in. It is best-effort: the pane options are the source of
// truth, and a closed tty or a terminal without support loses nothing.
func mirrorProgramStatus(st, pane, query string) {
	fields := strings.SplitN(query, "|", 4)
	if len(fields) != 4 || fields[0] != "on" {
		return
	}
	report := ProgramStatusReport(st, pane, fields[2], fields[3])
	if report == "" {
		return
	}
	// O_NOCTTY: hooks may have no controlling terminal, and opening the
	// pane's tty must not make it theirs.
	tty, err := os.OpenFile(fields[1], os.O_WRONLY|syscall.O_NOCTTY, 0)
	if err != nil {
		return
	}
	defer tty.Close()
	_, _ = tty.WriteString(passthrough(report))
}

// ProgramStatusReport builds the OSC 7501 report for one pane. Each pane is
// its own record (id=pane-N) because the outer terminal sees the whole tmux
// client as a single terminal; the root record is left to whatever runs
// tmux. Every report carries app and msg since a report replaces its record
// completely. It returns "" for states the protocol has no mapping for.
func ProgramStatusReport(st, pane, agent, task string) string {
	id := "id=pane-" + strings.TrimPrefix(pane, "%")
	if st == "clear" {
		return "\x1b]7501;state=clear:" + id + "\x1b\\"
	}
	var body string
	switch st {
	case "running":
		body = "state=working"
	case "blocked":
		body = "state=blocked:kind=permission"
	case "waiting":
		body = "state=blocked:kind=question"
	case "idle":
		body = "state=idle"
	default:
		return ""
	}
	body += ":" + id
	if a, ok := agents.ByName(agent); ok {
		body += ":app=" + a.App
	}
	// The spec discards a whole report whose text holds a control
	// character, so drop them rather than lose the state.
	msg := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, task)
	if msg != "" {
		body += ":msg=" + base64.StdEncoding.EncodeToString([]byte(msg))
	}
	return "\x1b]7501;" + body + "\x1b\\"
}

// passthrough wraps a sequence so tmux forwards it to the outer terminal
// instead of interpreting (and, for an unknown OSC, dropping) it.
func passthrough(seq string) string {
	return "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
}
