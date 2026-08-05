package agents

import "github.com/ahmedelgabri/tmux-agent-panel/plugins"

// The hook set lives in plugins/tap-claude/hooks/hooks.json — the same file
// the Claude Code plugin ships — so both install channels stay identical.
// Notably it has no --title-stdin: Claude's live pane title (glyph + task
// summary) is a better task source than the raw prompt, so the picker
// parses that instead.

func claudeSettingsPath() (string, error) {
	if dir, err := envOrHome("CLAUDE_CONFIG_DIR", ".claude"); err == nil {
		return dir + "/settings.json", nil
	} else {
		return "", pathErr("claude", err)
	}
}

func installClaude(self string) error {
	path, err := claudeSettingsPath()
	if err != nil {
		return err
	}
	return installHooks(path, self, plugins.ClaudeHooks)
}

func uninstallClaude() error {
	path, err := claudeSettingsPath()
	if err != nil {
		return err
	}
	return uninstallHooks(path)
}

func claudeInstalled() bool {
	path, err := claudeSettingsPath()
	return err == nil && hooksInstalled(path)
}
