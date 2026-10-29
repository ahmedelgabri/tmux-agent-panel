package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/agents"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/rex"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/tmux"
)

var (
	installFlags = map[string]*bool{}
	installForce bool
)

// checkInstallHost stops installs where the hooks would do nothing. They
// may still serve tmux sessions started later, hence --force.
func checkInstallHost() error {
	if installForce || tmux.InsideTmux() {
		return nil
	}
	if rex.InsideRex() {
		return errors.New("inside Rex, agents report their own state, so tap's hooks are not needed; pass --force to install them anyway (e.g. for tmux)")
	}
	return errors.New("not inside tmux, the only place tap's hooks report state; run this inside tmux, or pass --force to install anyway")
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Wire tap's state hooks into your coding agents",
	Long: `Installs the hooks that make agents report their state to tmux: merges
hook entries into Claude Code's settings.json and Codex's hooks.json
(identified by direct 'tap state' invocations, so re-running is idempotent),
and drops the pi extension into pi's extension directory. Install and
uninstall back up existing JSON configs and pi extensions before every
rewrite or managed removal, including byte-identical reinstalls, to
<file>.<YYYYMMDDTHHMMSS>.tap.bak. Same-second collisions add -1, -2, etc.
before .tap.bak. Backups are never overwritten or pruned.

By default only agents whose binary is on PATH are wired; pass --claude,
--codex, or --pi to force a specific subset. Files managed by Nix/Home
Manager are refused — wire those declaratively instead.

Only tmux needs the hooks: in Rex, agents report their own state. Install
refuses to run outside tmux unless --force is given.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkInstallHost(); err != nil {
			return err
		}
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
	installCmd.Flags().BoolVar(&installForce, "force", false, "install outside tmux")
	rootCmd.AddCommand(installCmd, uninstallCmd)
}
