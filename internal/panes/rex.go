package panes

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/rex"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/state"
)

// PickerPaneEnv carries the Rex pane the picker runs in. Unlike tmux popups,
// Rex popups are panes, so the listing has to skip the picker's own.
const PickerPaneEnv = "TAP_PICKER_PANE"

// rexScript reads every pane in one `rex do` round trip; a CLI call per pane
// and field would not fit the picker's 200 ms refresh.
//
//go:embed rex.lua
var rexScript string

// rexPane is one entry of rexScript's output.
type rexPane struct {
	BlockID     string `json:"block_id"`
	Session     string `json:"session"`
	Window      string `json:"window"`
	WindowIndex int    `json:"window_index"`
	PaneIndex   int    `json:"pane_index"`
	Command     string `json:"command"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	State       string `json:"state"`
	Kind        string `json:"kind"`
	App         string `json:"app"`
	Msg         string `json:"msg"`
}

func fetchRex() ([]Pane, error) {
	out, err := rex.Output("do", "-e", rexScript)
	if err != nil {
		return nil, err
	}
	return parseRex(out, os.Getenv(PickerPaneEnv))
}

// parseRex turns rexScript's output into the panes tmux's list-panes would
// give. Agents report their own state in Rex as program status records
// (see state.FromProgramStatus).
func parseRex(out, picker string) ([]Pane, error) {
	// Lua encodes an empty table as an object, so no panes arrive as {}.
	if strings.TrimSpace(out) == "{}" {
		return nil, nil
	}
	var entries []rexPane
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		return nil, fmt.Errorf("rex panes: %w", err)
	}
	var ps []Pane
	for _, e := range entries {
		if e.BlockID == picker {
			continue
		}
		p := Pane{
			ID:      e.BlockID,
			Addr:    fmt.Sprintf("%s:%d.%d", e.Session, e.WindowIndex, e.PaneIndex),
			Window:  e.Window,
			Command: e.Command,
			Path:    e.Path,
			Title:   e.Title,
		}
		// Any program can report status; only agents' records become agent
		// state, or a build tool's record would read as an orphaned agent.
		if a, ok := agents.ForApp(e.App); ok {
			p.Agent = a.Name
			p.State = state.FromProgramStatus(e.State, e.Kind)
			p.Task = e.Msg
		}
		ps = append(ps, p)
	}
	return ps, nil
}
