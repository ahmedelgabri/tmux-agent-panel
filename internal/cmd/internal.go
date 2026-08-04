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
		home, _ := os.UserHomeDir()
		out, err := panes.List(listAgentsOnly, home)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

var reloadCmd = &cobra.Command{
	Use:    "__reload",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		fmt.Println(picker.TransformReload(self))
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

func init() {
	listCmd.Flags().BoolVar(&listAgentsOnly, "agents", false, "agent panes only")
	rootCmd.AddCommand(listCmd, reloadCmd, toggleCmd)
}
