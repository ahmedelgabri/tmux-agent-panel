package agents

// Codex hook wiring (hooks.json shares Claude's schema). Codex has no
// Notification-equivalent event, so there is no waiting state;
// PermissionRequest maps straight to blocked. The prompt is the only task
// source, hence --title-stdin on UserPromptSubmit. Hooks load at session
// start only.
var codexEvents = map[string]string{
	"SessionStart":      "idle",
	"UserPromptSubmit":  "running --title-stdin",
	"PreToolUse":        "running",
	"Stop":              "idle",
	"PermissionRequest": "blocked",
	"SessionEnd":        "clear",
}

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
	return installHooks(path, self, codexEvents)
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
