// Package agents wires tap's state hooks into each coding agent's own
// configuration: Claude Code (settings.json hooks), Codex (hooks.json), and
// pi (a dropped-in extension file).
package agents

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Agent is one supported integration.
type Agent struct {
	Name string
	// ConfigPath is where the integration lives for this user.
	ConfigPath func() (string, error)
	// Install wires the hooks; self is the absolute tap binary path.
	Install func(self string) error
	// Uninstall removes only tap-owned pieces.
	Uninstall func() error
	// Installed reports whether tap's hooks are present.
	Installed func() bool
}

// All returns the supported agents in display order.
func All() []Agent {
	return []Agent{
		{
			Name:       "claude",
			ConfigPath: claudeSettingsPath,
			Install:    installClaude,
			Uninstall:  uninstallClaude,
			Installed:  claudeInstalled,
		},
		{
			Name:       "codex",
			ConfigPath: codexHooksPath,
			Install:    installCodex,
			Uninstall:  uninstallCodex,
			Installed:  codexInstalled,
		},
		{
			Name:       "pi",
			ConfigPath: piExtensionPath,
			Install:    installPi,
			Uninstall:  uninstallPi,
			Installed:  piInstalled,
		},
	}
}

// Detected reports whether the agent's binary is on PATH; install skips
// agents that aren't present unless explicitly requested.
func (a Agent) Detected() bool {
	_, err := exec.LookPath(a.Name)
	return err == nil
}

// Self resolves the running tap binary for embedding into hook commands.
func Self() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(self)
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
