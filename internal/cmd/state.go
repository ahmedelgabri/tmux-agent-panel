package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/state"
)

var (
	titleStdin bool
	titleText  string
	agentName  string
)

var stateCmd = &cobra.Command{
	Use:   "state <" + strings.Join(state.Names(), "|") + "|notification|clear>",
	Short: "Record agent state on the enclosing tmux pane",
	Long: `Records coding-agent activity in pane-scoped tmux user options
(@agent_state, @agent_task) so the picker can render agent rows. Meant to be
called from agent hooks; hooks run as children of the agent, so $TMUX_PANE
points at the right pane. Outside tmux this is a no-op.

'notification' derives the state from a Notification hook payload on stdin:
permission requests become blocked, everything else becomes waiting.
'clear' unsets all options.

--agent names the reporting agent (@agent_name) so the picker doesn't have
to rely on the pane's current command, which some systems misreport.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resolved, err := state.Set(args[0], agentName, titleStdin, titleText, os.Stdin)
		if err != nil {
			return err
		}
		// Hooks run with stdout piped and stay silent; only interactive
		// use gets feedback.
		if resolved != "" && stdoutIsTTY() {
			pane := os.Getenv("TMUX_PANE")
			if resolved == "clear" {
				fmt.Printf("%s %s\n", colorize(ansi.Bold, pane), colorize(ansi.Gray, "cleared"))
			} else {
				fmt.Printf("%s %s = %s\n", colorize(ansi.Bold, pane), state.StateOption, colorize(state.ByName(resolved).Color, resolved))
			}
		}
		return nil
	},
}

func init() {
	stateCmd.Flags().BoolVar(&titleStdin, "title-stdin", false,
		"store the hook JSON's .prompt from stdin as the pane's @agent_task")
	stateCmd.Flags().StringVar(&titleText, "title", "",
		"store this text (normalized) as the pane's @agent_task")
	stateCmd.Flags().StringVar(&agentName, "agent", "",
		"record this agent ("+strings.Join(agents.Names(), "|")+") as the pane's @agent_name")
	rootCmd.AddCommand(stateCmd)
}
