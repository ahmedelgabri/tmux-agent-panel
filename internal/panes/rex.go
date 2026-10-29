package panes

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/osc7501"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/rex"
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
	PaneTitle   string `json:"pane_title"`
	osc7501.Record
	Children []osc7501.Record `json:"children"`
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
// (see package osc7501).
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
			Title:   e.PaneTitle,
		}
		// The protocol cannot tell an agent from a build tool, so every
		// program that reports status shows as one; a record without a
		// title or an app names nothing to show.
		if e.App != "" || e.Title != "" {
			p.App = e.App
			p.AppTitle = e.Title
			p.Children = e.Children
			p.State = e.TapState()
			p.Task = e.Msg
		}
		ps = append(ps, p)
	}
	return ps, nil
}
