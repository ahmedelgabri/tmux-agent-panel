// Package agents wires tap's state hooks into each coding agent's own
// configuration: Claude Code (settings.json hooks), Codex (hooks.json), and
// pi (a dropped-in extension file).
package agents

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
)

// Agent is one supported integration: how it installs, and how the picker
// renders it. Everything that varies per agent lives here so adding one is
// a single-entry change.
type Agent struct {
	Name string
	// Icon is the picker glyph and Color its ANSI slot, distinct per agent:
	// Claude yellow (closest named slot to its orange), Codex cyan, pi
	// magenta. The picker composes them so the agent-name column can share
	// the color.
	Icon  string
	Color string
	// TaskFromTitle marks agents that publish a live task summary in the
	// pane title (behind a status glyph) — a better task source than the
	// raw prompt, so their hooks don't store @agent_task and the picker
	// parses the title instead.
	TaskFromTitle bool
	// ConfigPath is where the integration lives for this user.
	ConfigPath func() (string, error)
	// Install wires the hooks; they invoke `tap` from PATH.
	Install func() error
	// Uninstall removes only tap-owned pieces.
	Uninstall func() error
	// Installed reports whether tap's hooks are present.
	Installed func() bool
	// Current reports whether the installed hooks match the embedded set;
	// false means a stale install whose commands predate a flag change
	// (fix: `tap install`). Only meaningful when Installed.
	Current func() bool
}

// All returns the supported agents in display order.
func All() []Agent {
	return []Agent{
		{
			Name:          "claude",
			Icon:          "✳",
			Color:         ansi.Yellow,
			TaskFromTitle: true,
			ConfigPath:    claudeSettingsPath,
			Install:       installClaude,
			Uninstall:     uninstallClaude,
			Installed:     claudeInstalled,
			Current:       claudeCurrent,
		},
		{
			Name:       "codex",
			Icon:       "⌬",
			Color:      ansi.Cyan,
			ConfigPath: codexHooksPath,
			Install:    installCodex,
			Uninstall:  uninstallCodex,
			Installed:  codexInstalled,
			Current:    codexCurrent,
		},
		{
			Name:       "pi",
			Icon:       "π",
			Color:      ansi.Magenta,
			ConfigPath: piExtensionPath,
			Install:    installPi,
			Uninstall:  uninstallPi,
			Installed:  piInstalled,
			Current:    piCurrent,
		},
	}
}

// Names lists the supported agent names in display order.
func Names() []string {
	all := All()
	names := make([]string, len(all))
	for i, a := range all {
		names[i] = a.Name
	}
	return names
}

// ByName returns the agent with the given name.
func ByName(name string) (Agent, bool) {
	for _, a := range All() {
		if a.Name == name {
			return a, true
		}
	}
	return Agent{}, false
}

// ForCommand maps a pane's current command to its agent. Home Manager
// wraps binaries, so agents can show up as e.g. ".claude-wrapped" — strip
// the wrapper to detect them.
func ForCommand(command string) (Agent, bool) {
	c := strings.TrimPrefix(command, ".")
	c = strings.TrimSuffix(c, "-wrapped")
	return ByName(c)
}

// Detected reports whether the agent's binary is on PATH; install skips
// agents that aren't present unless explicitly requested.
func (a Agent) Detected() bool {
	_, err := exec.LookPath(a.Name)
	return err == nil
}

func homePath(parts ...string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{home}, parts...)...), nil
}

func envOrHome(env string, parts ...string) (string, error) {
	if dir := os.Getenv(env); dir != "" {
		return dir, nil
	}
	return homePath(parts...)
}

func pathErr(name string, err error) error {
	return fmt.Errorf("%s: resolving config path: %w", name, err)
}
