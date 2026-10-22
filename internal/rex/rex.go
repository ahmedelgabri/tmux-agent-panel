// Package rex is a thin wrapper around the rex CLI, the Rex counterpart of
// package tmux. Rex terminals carry REX_SERVER and REX_BLOCK, which play the
// roles of $TMUX and $TMUX_PANE.
package rex

import (
	"os"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/command"
)

// InsideRex reports whether the process runs in a Rex terminal. tmux running
// inside Rex owns its own panes, so tmux wins when both are present.
func InsideRex() bool {
	return os.Getenv("REX_BLOCK") != "" && os.Getenv("TMUX") == ""
}

// Block is the Rex block (pane) of the current terminal.
func Block() string {
	return os.Getenv("REX_BLOCK")
}

// Run executes a rex command, discarding output.
func Run(args ...string) error {
	return command.Run("rex", globalArgs(args)...)
}

// Output executes a rex command and returns its stdout without the trailing
// newline.
func Output(args ...string) (string, error) {
	return command.Output("rex", globalArgs(args)...)
}

// globalArgs pins every call to the server the terminal belongs to: some rex
// builds ignore REX_SERVER and fall back to the default server. tap only
// reads and acts on a running server, so it never starts one.
func globalArgs(args []string) []string {
	global := []string{"--autostart=false"}
	if server := os.Getenv("REX_SERVER"); server != "" {
		global = append(global, "-S", server)
	}
	return append(global, args...)
}
