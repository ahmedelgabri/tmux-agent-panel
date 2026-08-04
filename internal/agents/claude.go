package agents

// Claude Code hook wiring. No --title-stdin: Claude's live pane title
// (glyph + task summary) is a better task source than the raw prompt, so
// the picker parses that instead. Notification routes to blocked/waiting
// based on the payload.
var claudeEvents = map[string]string{
	"SessionStart":     "idle",
	"UserPromptSubmit": "running",
	"PreToolUse":       "running",
	"Stop":             "idle",
	"Notification":     "notification",
	"SessionEnd":       "clear",
}

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
	return installHooks(path, self, claudeEvents)
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
