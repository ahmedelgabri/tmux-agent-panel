package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/picker"
)

var inPopup bool

var pickCmd = &cobra.Command{
	Use:   "pick",
	Short: "Open the pane picker",
	Long: `Opens the agent-aware pane picker in a tmux popup. Agent panes are
grouped first, sorted blocked → waiting → running → idle, and the list
live-refreshes while open. Enter switches to the selected pane; ctrl-a
toggles an agents-only view; ctrl-x/ctrl-w/ctrl-q kill the highlighted
pane/window/session.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return picker.Run(inPopup)
	},
}

func init() {
	// The popup re-executes `tap pick --in-popup`; users never pass this.
	pickCmd.Flags().BoolVar(&inPopup, "in-popup", false, "")
	_ = pickCmd.Flags().MarkHidden("in-popup")
	rootCmd.AddCommand(pickCmd)
}
