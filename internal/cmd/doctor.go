package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check tap's runtime dependencies and hook wiring",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		failed := false
		for _, c := range agents.Doctor() {
			mark := "✓"
			if !c.OK {
				mark = "✗"
				failed = true
			}
			fmt.Printf("%s %-8s %s\n", mark, c.Name, c.Detail)
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
