package agents

import (
	"path/filepath"

	"github.com/ahmedelgabri/tmux-agent-panel/plugins"
)

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
	return filepath.Join(dir, "hooks.json"), nil
}

var installCodex, uninstallCodex, codexInstalled, codexCurrent = jsonHookFuncs(codexHooksPath, plugins.CodexHooks)
