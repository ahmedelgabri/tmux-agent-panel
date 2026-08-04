package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "tap",
	Short: "Tmux Agent Panel — agent-aware tmux pane picker",
	Long: `tap lists every tmux pane in an fzf picker with live coding-agent status
(Claude Code, Codex, pi): which agent runs where, whether it is blocked on a
permission request, waiting for input, running, or idle, and what it is
working on.

Agents report state through hooks that tap installs into their own configs
('tap install'); state lives in pane-scoped tmux user options, and the
picker stays live while open.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

// Execute runs the CLI.
func Execute() {
	rootCmd.Version = Version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "tap:", err)
		os.Exit(1)
	}
}
