package agents

import "github.com/ahmedelgabri/tmux-agent-panel/plugins"

// The hook set lives in plugins/tap-codex/hooks/hooks.json — the same file
// the Codex plugin ships — so both install channels stay identical. Codex
// has no Notification-equivalent event, so there is no waiting state;
// PermissionRequest maps straight to blocked, and the prompt is the only
// task source (--title-stdin on UserPromptSubmit). Hooks load at session
// start only.

func codexHooksPath() (string, error) {
	dir, err := envOrHome("CODEX_HOME", ".codex")
	if err != nil {
		return "", pathErr("codex", err)
	}
	return dir + "/hooks.json", nil
}

func installCodex(self string) error {
	path, err := codexHooksPath()
	if err != nil {
		return err
	}
	return installHooks(path, self, plugins.CodexHooks)
}

func uninstallCodex() error {
	path, err := codexHooksPath()
	if err != nil {
		return err
	}
	return uninstallHooks(path)
}

func codexInstalled() bool {
	path, err := codexHooksPath()
	return err == nil && hooksInstalled(path)
}
