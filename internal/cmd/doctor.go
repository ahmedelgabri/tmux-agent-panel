package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check tap's runtime dependencies and hook wiring",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		failed := false
		for _, c := range agents.Doctor() {
			mark := colorize(ansi.Green, "✓")
			detail := colorize(ansi.Gray, c.Detail)
			if !c.OK {
				mark = colorize(ansi.Red, "✗")
				detail = colorize(ansi.Red, c.Detail)
				failed = true
			}
			fmt.Printf("%s %s %s\n", mark, colorize(ansi.Bold, fmt.Sprintf("%-8s", c.Name)), detail)
		}
		if failed {
			return fmt.Errorf("some checks failed")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
