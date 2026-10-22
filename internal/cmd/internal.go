package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/panes"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/picker"
)

// Hidden plumbing used by the picker's fzf bindings: reload commands and
// transforms re-execute tap itself, so these must exist as subcommands even
// though users never call them.

var listAgentsOnly bool

var listCmd = &cobra.Command{
	Use:    "__list",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// fzf exports FZF_PROMPT to reload children, so plain reloads
		// preserve the active view; --agents is for actions that set the
		// view explicitly (the ctrl-a toggle).
		agentsOnly := listAgentsOnly || os.Getenv("FZF_PROMPT") == picker.PromptAgents
		home, _ := os.UserHomeDir()
		out, err := panes.List(agentsOnly, home)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

var toggleCmd = &cobra.Command{
	Use:    "__toggle",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		fmt.Println(picker.TransformToggle(self))
		return nil
	},
}

// Rex's kill bindings need the session and window of a block, which only
// the rex CLI can look up; tmux's bindings call tmux directly.
var killCmd = &cobra.Command{
	Use:    "__kill <pane|window|session> <block>",
	Hidden: true,
	Args:   cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return picker.KillRex(args[0], args[1])
	},
}

// Rex's preview binding needs its capture rewritten before fzf sees it;
// tmux's binding calls tmux directly.
var previewCmd = &cobra.Command{
	Use:    "__preview <block>",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := picker.PreviewRex(args[0])
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVar(&listAgentsOnly, "agents", false, "agent panes only")
	rootCmd.AddCommand(listCmd, toggleCmd, killCmd, previewCmd)
}
