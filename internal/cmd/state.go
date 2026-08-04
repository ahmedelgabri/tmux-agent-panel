package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/state"
)

var titleStdin bool

var stateCmd = &cobra.Command{
	Use:   "state <running|idle|waiting|blocked|notification|clear>",
	Short: "Record agent state on the enclosing tmux pane",
	Long: `Records coding-agent activity in pane-scoped tmux user options
(@agent_state, @agent_task) so the picker can render agent rows. Meant to be
called from agent hooks; hooks run as children of the agent, so $TMUX_PANE
points at the right pane. Outside tmux this is a no-op.

'notification' derives the state from a Notification hook payload on stdin:
permission requests become blocked, everything else becomes waiting.
'clear' unsets both options.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resolved, err := state.Set(args[0], titleStdin, os.Stdin)
		if err != nil {
			return err
		}
		// Hooks run with stdout piped and stay silent; only interactive
		// use gets feedback.
		if resolved != "" && stdoutIsTTY() {
			pane := os.Getenv("TMUX_PANE")
			if resolved == "clear" {
				fmt.Printf("%s %s\n", colorize(ansiBold, pane), colorize(ansiGray, "cleared"))
			} else {
				fmt.Printf("%s @agent_state = %s\n", colorize(ansiBold, pane), colorize(stateColor(resolved), resolved))
			}
		}
		return nil
	},
}

func init() {
	stateCmd.Flags().BoolVar(&titleStdin, "title-stdin", false,
		"store the hook JSON's .prompt from stdin as the pane's @agent_task")
	rootCmd.AddCommand(stateCmd)
}
