package picker

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/panes"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/rex"
)

// rexActions runs on Rex block IDs. Closing a window or session needs IDs
// the rows don't carry, so the kill bindings go through `tap __kill`.
func rexActions(self string) paneActions {
	kill := func(kind string) string {
		return shellQuote(self) + " __kill " + kind + " {1}"
	}
	return paneActions{
		preview:     "[ -n {1} ] && " + shellQuote(self) + " __preview {1} || true",
		killPane:    "[ -n {1} ] && " + kill("pane"),
		killWindow:  confirm("window", kill("window")),
		killSession: confirm("session", kill("session")),
	}
}

// openRexPopup opens the picker in a floating pane over the active window,
// the counterpart of tmux's display-popup. It closes when the picker exits.
// The popup's REX_BLOCK names the popup itself, so the opener's pane rides
// along in CurrentPaneEnv.
func openRexPopup(self string) error {
	args, err := json.Marshal(map[string]any{
		"bounds": []float64{0.075, 0.075, 0.85, 0.85},
		"focus":  true,
		"layout": map[string]any{"block": map[string]any{
			"flavor": "com.superlogical.terminal.shell",
			"options": map[string]any{
				"command": []string{"env", panes.CurrentPaneEnv + "=" + rex.Block(), self, "pick", "--in-popup"},
			},
		}},
	})
	if err != nil {
		return err
	}
	return rex.Run("api", "call", "-s", os.Getenv("REX_SESSION"), "session.new_layer", string(args))
}

// enterRexPopup prepares the picker's own process. The listing skips the
// popup's pane, and without REX_SESSION the rex CLI resolves block IDs in
// every session rather than only the popup's, which the preview and kill
// bindings rely on.
func enterRexPopup() {
	os.Setenv(panes.PickerPaneEnv, rex.Block())
	os.Unsetenv("REX_SESSION")
}

type rexTarget struct {
	SessionID string `json:"session_id"`
	Block     struct {
		WindowID string `json:"window_id"`
	} `json:"block"`
}

func locate(block string) (rexTarget, error) {
	var t rexTarget
	out, err := rex.Output("block", "inspect", block, "--json")
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal([]byte(out), &t); err != nil {
		return t, fmt.Errorf("rex block %s: %w", block, err)
	}
	return t, nil
}

// switchRex shows the block in the Rex app. Focusing it makes its window
// active on the server, but the app keeps showing the tab it had selected,
// so the app is asked to select the window as well.
func switchRex(block string) error {
	t, err := locate(block)
	if err != nil {
		return err
	}
	if err := rex.Run("focus", "-s", t.SessionID, block); err != nil {
		return err
	}
	return rex.Run("do", "session.select", "session_id="+t.SessionID, "window_id="+t.Block.WindowID)
}

// Rex's VT capture writes runs of blank cells as cursor-forward sequences,
// which fzf's preview drops along with the gap they stand for.
var cursorForward = regexp.MustCompile(`\x1b\[([0-9]*)C`)

// PreviewRex returns the block's screen and scrollback with colors, with
// cursor-forward sequences turned back into the spaces they skip.
func PreviewRex(block string) (string, error) {
	out, err := rex.Output("capture", "-b", block, "--format=vt")
	if err != nil {
		return "", err
	}
	return expandCursorForward(out), nil
}

func expandCursorForward(s string) string {
	return cursorForward.ReplaceAllStringFunc(s, func(seq string) string {
		// An omitted or zero count moves one cell, as in any terminal.
		n, _ := strconv.Atoi(cursorForward.FindStringSubmatch(seq)[1])
		return strings.Repeat(" ", max(n, 1))
	})
}

// KillRex closes the pane, window, or session that holds block.
func KillRex(kind, block string) error {
	if kind == "pane" {
		return rex.Run("block", "close", block)
	}
	t, err := locate(block)
	if err != nil {
		return err
	}
	switch kind {
	case "window":
		return rex.Run("window", "close", "-s", t.SessionID, t.Block.WindowID)
	case "session":
		return rex.Run("kill", t.SessionID)
	}
	return fmt.Errorf("invalid kind %q (want pane|window|session)", kind)
}
