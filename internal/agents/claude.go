package agents

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ahmedelgabri/tmux-agent-panel/plugins"
)

// The hook set lives in plugins/tap-claude/hooks/hooks.json — the same file
// the Claude Code plugin ships — so both install channels stay identical.
// Notably it has no --title-stdin: Claude's live pane title (glyph + task
// summary) is a better task source than the raw prompt, so the picker
// parses that instead.

// claudeMinVersion is the oldest Claude Code safe to install into. Before
// 2.1.101 an unrecognized hook event name made Claude ignore the entire
// settings.json — permissions included — so the minimum must be a version
// that either recognizes every event tap wires or (2.1.101+) safely
// ignores unknown ones. Every wired event exists by 2.1.78 (StopFailure,
// the newest; verified against the 2.1.78 npm bundle). If a future hook
// set adds an event introduced after 2.1.101, bump this back to 2.1.101.
var claudeMinVersion = [3]int{2, 1, 78}

func claudeSettingsPath() (string, error) {
	dir, err := envOrHome("CLAUDE_CONFIG_DIR", ".claude")
	if err != nil {
		return "", pathErr("claude", err)
	}
	return filepath.Join(dir, "settings.json"), nil
}

// parseVersion reads the leading dotted triple from `claude --version`
// output ("2.1.226 (Claude Code)").
func parseVersion(out string) ([3]int, bool) {
	var v [3]int
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return v, false
	}
	parts := strings.SplitN(fields[0], ".", 3)
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		v[i] = n
	}
	return v, true
}

func versionLess(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// claudeVersionSupported refuses installs into a Claude Code old enough to
// choke on the hook set. No binary or unparseable output passes: forced
// installs must work without the binary on PATH, and a wrapper mangling
// --version shouldn't block anyone — the check is best-effort protection.
func claudeVersionSupported() error {
	out, err := exec.Command("claude", "--version").Output()
	if err != nil {
		return nil
	}
	v, ok := parseVersion(string(out))
	if !ok || !versionLess(v, claudeMinVersion) {
		return nil
	}
	return fmt.Errorf("claude %d.%d.%d is older than %d.%d.%d, which ignores the entire settings.json when it sees an unknown hook event; upgrade Claude Code first",
		v[0], v[1], v[2], claudeMinVersion[0], claudeMinVersion[1], claudeMinVersion[2])
}

var installClaudeHooks, uninstallClaude, claudeInstalled, claudeCurrent = jsonHookFuncs(claudeSettingsPath, plugins.ClaudeHooks)

func installClaude() error {
	if err := claudeVersionSupported(); err != nil {
		return err
	}
	return installClaudeHooks()
}
