package agents

import (
	"path/filepath"

	"github.com/ahmedelgabri/tmux-agent-panel/plugins"
)

// The hook set lives in plugins/tap-claude/hooks/hooks.json — the same file
// the Claude Code plugin ships — so both install channels stay identical.
// Notably it has no --title-stdin: Claude's live pane title (glyph + task
// summary) is a better task source than the raw prompt, so the picker
// parses that instead.

func claudeSettingsPath() (string, error) {
	dir, err := envOrHome("CLAUDE_CONFIG_DIR", ".claude")
	if err != nil {
		return "", pathErr("claude", err)
	}
	return filepath.Join(dir, "settings.json"), nil
}

var installClaude, uninstallClaude, claudeInstalled, claudeCurrent = jsonHookFuncs(claudeSettingsPath, plugins.ClaudeHooks)
