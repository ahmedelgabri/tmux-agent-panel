package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
)

var installFlags = map[string]*bool{}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Wire tap's state hooks into your coding agents",
	Long: `Installs the hooks that make agents report their state to tmux: merges
hook entries into Claude Code's settings.json and Codex's hooks.json
(identified by direct 'tap state' invocations, so re-running is idempotent),
and drops the pi extension into pi's extension directory. Install and
uninstall back up existing JSON configs before every rewrite, including
byte-identical reinstalls, to <file>.<YYYYMMDDTHHMMSS>.tap.bak. Same-second
collisions add -1, -2, etc. before .tap.bak. Backups are never overwritten
or pruned. The pi extension is overwritten without a backup.

By default only agents whose binary is on PATH are wired; pass --claude,
--codex, or --pi to force a specific subset. Files managed by Nix/Home
Manager are refused — wire those declaratively instead.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachSelected(func(a agents.Agent) error {
			return a.Install()
		}, "installed")
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove tap's hooks from your coding agents",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachSelected(func(a agents.Agent) error {
			return a.Uninstall()
		}, "removed")
	},
}

// forEachSelected applies fn to the agents picked via flags, or to all
// detected agents when no flag is given. Failures don't stop the other
// agents; the first error is reported after all ran.
func forEachSelected(fn func(agents.Agent) error, verb string) error {
	anyFlag := false
	for _, set := range installFlags {
		anyFlag = anyFlag || *set
	}

	var firstErr error
	for _, a := range agents.All() {
		if anyFlag {
			if !*installFlags[a.Name] {
				continue
			}
		} else if !a.Detected() {
			fmt.Printf("%-8s skipped (binary not on PATH; force with --%s)\n", a.Name, a.Name)
			continue
		}
		path, _ := a.ConfigPath()
		if err := fn(a); err != nil {
			fmt.Fprintf(os.Stderr, "%-8s failed: %v\n", a.Name, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Printf("%-8s %s (%s)\n", a.Name, verb, path)
	}
	return firstErr
}

func init() {
	// One shared *bool per agent, registered on both commands.
	for _, a := range agents.All() {
		flag := new(bool)
		installFlags[a.Name] = flag
		for _, c := range []*cobra.Command{installCmd, uninstallCmd} {
			c.Flags().BoolVar(flag, a.Name, false, "only "+a.Name)
		}
	}
	rootCmd.AddCommand(installCmd, uninstallCmd)
}
