package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/panes"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/state"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check tap's runtime dependencies and hook wiring",
	Long: `Checks runtime dependencies, user-scoped direct hooks and native
plugin/package installs, and pane state on the current tmux server.
Project-scoped installs, managed settings, and one-session agent CLI
overrides are not checked. Native installs are inspected without running
an agent or modifying its package cache.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		checks := agents.Doctor()
		// Orphaned agent state is runtime breakage the config checks can't
		// see; a Fetch error just means no tmux server to scan.
		if ps, err := panes.Fetch(); err == nil {
			for _, p := range ps {
				if panes.Orphaned(p) {
					checks = append(checks, agents.Check{
						Name: "pane",
						OK:   false,
						Detail: fmt.Sprintf("%s (%s): %s=%s but no agent identity — hooks not passing --agent, or stale options (run `tap state clear` in that pane)",
							p.ID, p.Addr, state.StateOption, p.State),
					})
				}
			}
		}
		failed := false
		for _, c := range checks {
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
