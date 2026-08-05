package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedelgabri/tmux-agent-panel/plugins"
)

// existing simulates a user settings.json with hooks tap must not touch.
const existing = `{
	"theme": "auto",
	"hooks": {
		"SessionStart": [
			{
				"hooks": [
					{"type": "command", "command": "~/.claude/hooks/log-event.sh SessionStart", "async": true}
				]
			}
		]
	}
}`

// testHooks mirrors the embedded plugin hooks.json shape.
var testHooks = []byte(`{
	"hooks": {
		"SessionStart": [
			{"hooks": [{"type": "command", "command": "tap state idle", "async": true}]}
		],
		"Stop": [
			{"hooks": [{"type": "command", "command": "tap state idle", "async": true}]}
		]
	}
}`)

func read(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	return m
}

func TestInstallPreservesForeignHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}

	m := read(t, path)
	if m["theme"] != "auto" {
		t.Errorf("unrelated settings must survive")
	}
	raw, _ := json.Marshal(m)
	if !strings.Contains(string(raw), "log-event.sh SessionStart") {
		t.Errorf("pre-existing hook was lost")
	}
	if !strings.Contains(string(raw), "tap state idle") {
		t.Errorf("tap hook missing")
	}
	if !hooksInstalled(path) {
		t.Errorf("hooksInstalled should be true after install")
	}
	if _, err := os.Stat(path + ".tap.bak"); err != nil {
		t.Errorf("backup missing: %v", err)
	}
}

func TestInstallIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}
	once, _ := os.ReadFile(path)
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}
	twice, _ := os.ReadFile(path)
	if string(once) != string(twice) {
		t.Errorf("second install changed the file:\n%s\nvs\n%s", once, twice)
	}
}

func TestInstallCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "hooks.json")
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}
	m := read(t, path)
	if _, ok := m["hooks"]; !ok {
		t.Errorf("hooks key missing in created file")
	}
}

func TestUninstallRemovesOnlyOurs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}
	if err := uninstallHooks(path); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), Marker) {
		t.Errorf("tap entries remain after uninstall:\n%s", data)
	}
	if !strings.Contains(string(data), "log-event.sh SessionStart") {
		t.Errorf("foreign hook removed by uninstall:\n%s", data)
	}
	m := read(t, path)
	hooks := m["hooks"].(map[string]any)
	if _, ok := hooks["Stop"]; ok {
		t.Errorf("event emptied by uninstall should be dropped")
	}
}

func TestInstallKeepsCommandsPathBased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	// Commands stay `tap ...` verbatim: absolute paths go stale when the
	// binary moves (Nix store paths change every rebuild).
	if !strings.Contains(string(data), `"command": "tap state idle"`) {
		t.Errorf("command must invoke tap from PATH:\n%s", data)
	}
}

func TestInstallPreservesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-settings.json")
	link := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := installHooks(link, testHooks); err != nil {
		t.Fatal(err)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("install replaced the symlink with a regular file")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), Marker) {
		t.Errorf("hooks must land in the symlink target:\n%s", data)
	}

	if err := uninstallHooks(link); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("uninstall replaced the symlink with a regular file")
	}
}

// The embedded plugin files are the single source of truth for installs;
// every command must carry the marker so idempotent re-install, uninstall,
// and the PATH rewrite all keep working when the files change.
func TestEmbeddedPluginHooksAreCanonical(t *testing.T) {
	for name, data := range map[string][]byte{
		"claude": plugins.ClaudeHooks,
		"codex":  plugins.CodexHooks,
	} {
		var p pluginHooks
		if err := json.Unmarshal(data, &p); err != nil {
			t.Fatalf("%s: embedded hooks.json invalid: %v", name, err)
		}
		if len(p.Hooks) == 0 {
			t.Fatalf("%s: embedded hooks.json has no events", name)
		}
		for event, groups := range p.Hooks {
			for _, g := range groups {
				group := g.(map[string]any)
				for _, e := range group["hooks"].([]any) {
					entry := e.(map[string]any)
					cmd, _ := entry["command"].(string)
					if !strings.HasPrefix(cmd, Marker) {
						t.Errorf("%s/%s: command %q must start with %q", name, event, cmd, Marker)
					}
					if async, _ := entry["async"].(bool); !async {
						t.Errorf("%s/%s: command %q must be async", name, event, cmd)
					}
				}
			}
		}
	}
}

func TestUninstallMissingFileIsNoop(t *testing.T) {
	if err := uninstallHooks(filepath.Join(t.TempDir(), "nope.json")); err != nil {
		t.Errorf("missing file should be a no-op, got %v", err)
	}
}

func TestRefusesInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(path, testHooks); err == nil {
		t.Errorf("install should refuse invalid JSON")
	}
	if err := uninstallHooks(path); err == nil {
		t.Errorf("uninstall should refuse invalid JSON")
	}
}

func TestRefusesUnwritableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(existing), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(path, testHooks); err == nil {
		t.Errorf("install should refuse a read-only file")
	}
}

func TestPiInstallRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)

	if err := installPi(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "extensions", "tap-agent-state.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), piMarker) {
		t.Errorf("installed extension must carry the tap marker")
	}
	if !piInstalled() {
		t.Errorf("piInstalled should be true after install")
	}

	if err := uninstallPi(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("extension should be removed by uninstall")
	}
}

func TestPiUninstallSparesForeignFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	path := filepath.Join(dir, "extensions", "tap-agent-state.ts")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("// hand-written"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := uninstallPi(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("uninstall must not delete a file it does not own")
	}
}

func TestClaudeCodexPathsRespectEnv(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/claude-test")
	if p, _ := claudeSettingsPath(); p != "/tmp/claude-test/settings.json" {
		t.Errorf("claude path = %q", p)
	}
	t.Setenv("CODEX_HOME", "/tmp/codex-test")
	if p, _ := codexHooksPath(); p != "/tmp/codex-test/hooks.json" {
		t.Errorf("codex path = %q", p)
	}
}
