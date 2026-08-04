package agents

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

// pi has no hook config file; integration is an extension dropped into the
// directory pi auto-discovers. The extension reports state itself (no
// `tap state` calls), so it works even when tap moves. `/reload` inside pi
// hot-loads it; otherwise it's picked up on the next session.
//
//go:embed pi_extension.ts
var piExtension []byte

// piMarker identifies the extension as tap-owned so uninstall never deletes
// a user's hand-written extension of the same name.
const piMarker = "Managed by tap (tmux-agent-panel)"

func piExtensionPath() (string, error) {
	dir, err := envOrHome("PI_CODING_AGENT_DIR", ".config", "pi", "agent")
	if err != nil {
		return "", pathErr("pi", err)
	}
	return filepath.Join(dir, "extensions", "tap-agent-state.ts"), nil
}

func installPi(_ string) error {
	path, err := piExtensionPath()
	if err != nil {
		return err
	}
	if err := checkWritable(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, piExtension, 0o644)
}

func uninstallPi() error {
	path, err := piExtensionPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.Contains(string(data), piMarker) {
		return nil
	}
	return os.Remove(path)
}

func piInstalled() bool {
	path, err := piExtensionPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), piMarker)
}
