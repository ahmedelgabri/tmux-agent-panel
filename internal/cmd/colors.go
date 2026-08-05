package cmd

import (
	"os"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
)

func colorize(color, s string) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return color + s + ansi.Reset
}

// stdoutIsTTY separates interactive use from hook invocations: hooks get
// silent plumbing, humans get feedback.
func stdoutIsTTY() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
