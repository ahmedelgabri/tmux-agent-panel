package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ahmedelgabri/tmux-agent-panel/plugins"
)

const Marker = "tap state"

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
	if got := backups(t, path); !slices.Equal(got, []string{existing}) {
		t.Errorf("backups = %q, want original settings", got)
	}
}

func TestOwnsCommand(t *testing.T) {
	for command, want := range map[string]bool{
		"tap state idle --agent claude":              true,
		"/nix/store/hash-tap/bin/tap state clear":    true,
		"'/home/a b/bin/tap' state running":          true,
		`"/home/a b/bin/tap" state running`:          true,
		"exec /usr/local/bin/tap state notification": true,
		"tap\tstate\tidle":                           true,
		`notify-send "tap state changed"`:            false,
		"echo tap state idle":                        false,
		"not-tap state idle":                         false,
		"tap states idle":                            false,
		"tap state":                                  false,
		"tap state idle; echo user-work":             false,
		"tap state idle && echo user-work":           false,
		"tap state idle | logger":                    false,
		"tap state idle\necho user-work":             false,
		"tap state idle > /tmp/user-log":             false,
		"'tap state idle":                            false,
	} {
		if got := ownsCommand(command); got != want {
			t.Errorf("ownsCommand(%q) = %v, want %v", command, got, want)
		}
	}
}

// Compare contents rather than filenames: counter suffixes do not sort by age.
func backups(t *testing.T, path string) []string {
	t.Helper()
	names, err := filepath.Glob(path + ".*.tap.bak")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(data))
	}
	slices.Sort(out)
	return out
}

func TestBackupBeforeEveryModification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		run    func() error
		backup bool
	}{
		{func() error { return installHooks(path, testHooks) }, true},
		{func() error { return installHooks(path, testHooks) }, true},
		{func() error { return os.WriteFile(path, []byte(`{"theme": "dark", "hooks": {}}`), 0o644) }, false},
		{func() error { return installHooks(path, testHooks) }, true},
		{func() error { return uninstallHooks(path) }, true},
	}
	var want []string
	for _, step := range steps {
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := step.run(); err != nil {
			t.Fatal(err)
		}
		if step.backup {
			want = append(want, string(before))
			slices.Sort(want)
		}
		if got := backups(t, path); !slices.Equal(got, want) {
			t.Fatalf("backups = %q, want %q", got, want)
		}
	}
}

func TestPiBackupBeforeEveryModification(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	path := filepath.Join(dir, "extensions", "tap-agent-state.ts")
	put(t, path, []byte("// hand-written"))
	steps := []struct {
		run    func() error
		backup bool
	}{
		{installPi, true},
		{installPi, true},
		{func() error { return os.WriteFile(path, []byte(piMarker+"\n// hand edit"), 0o644) }, false},
		{installPi, true},
		{uninstallPi, true},
	}
	var want []string
	for _, step := range steps {
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := step.run(); err != nil {
			t.Fatal(err)
		}
		if step.backup {
			want = append(want, string(before))
			slices.Sort(want)
		}
		if got := backups(t, path); !slices.Equal(got, want) {
			t.Fatalf("backups = %q, want %q", got, want)
		}
	}
}

func TestBackupSameSecond(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		stamp := time.Now().Format("20060102T150405")
		names := []string{
			path + "." + stamp + ".tap.bak",
			path + "." + stamp + "-1.tap.bak",
			path + "." + stamp + "-2.tap.bak",
		}
		for _, name := range names {
			if err := backup(path, []byte(name)); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range names {
			data, err := os.ReadFile(name)
			if err != nil || string(data) != name {
				t.Fatalf("backup %s: %q, %v", name, data, err)
			}
			info, err := os.Stat(name)
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("backup must be private: %v, %v", info, err)
			}
		}
	})
}

func TestInstallPreservesHookMentions(t *testing.T) {
	const foreign = `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"notify-send 'tap state changed'"},{"type":"command","command":"tap state idle; echo user-work"}]}]}}`
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	if hooksInstalled(path) {
		t.Fatal("mentions must not count as installed hooks")
	}
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}
	if !hooksCurrent(path, testHooks) {
		t.Fatal("mentions must not affect freshness")
	}
	if err := uninstallHooks(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != foreign {
		t.Fatalf("foreign hooks changed: %s, %v", data, err)
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

func TestInstallPreservesFormattingAndRoundTrips(t *testing.T) {
	// Deliberately non-alphabetical keys, two-space indent, and characters
	// encoding/json would re-escape — everything the old map round-trip
	// used to normalize.
	const spaced = `{
  "zeta": "é <ok>",
  "theme": "auto",
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {"type": "command", "command": "~/.claude/hooks/log-event.sh", "async": true}
        ]
      }
    ]
  }
}
`
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(spaced), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "\"zeta\": \"é <ok>\",\n  \"theme\": \"auto\",") {
		t.Errorf("user content must keep its exact bytes:\n%s", data)
	}
	if !strings.Contains(string(data), "tap state idle") {
		t.Errorf("tap hook missing:\n%s", data)
	}

	if err := uninstallHooks(path); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != spaced {
		t.Errorf("install+uninstall must restore the original bytes:\n%s", after)
	}
}

func TestInstallMigratesDroppedEvents(t *testing.T) {
	// An install written by a hook set that still hooked PostToolUse — an
	// event the current embedded set no longer contains.
	const older = `{
	"hooks": {
		"PostToolUse": [
			{"hooks": [{"type": "command", "command": "tap state running", "async": true}]}
		],
		"SessionStart": [
			{
				"hooks": [
					{"type": "command", "command": "~/.claude/hooks/log-event.sh SessionStart", "async": true}
				]
			}
		]
	}
}`
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(older), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}

	m := read(t, path)
	hooks := m["hooks"].(map[string]any)
	if _, ok := hooks["PostToolUse"]; ok {
		t.Errorf("tap entry under a dropped event must be migrated away")
	}
	raw, _ := json.Marshal(m)
	if !strings.Contains(string(raw), "log-event.sh SessionStart") {
		t.Errorf("foreign hook was lost during migration")
	}
	if !hooksCurrent(path, testHooks) {
		t.Errorf("migrated install should be current")
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

func TestHooksCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}
	if !hooksCurrent(path, testHooks) {
		t.Errorf("fresh install should be current")
	}

	// A stale install: same events, but the commands predate a flag.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := strings.ReplaceAll(string(data), "tap state idle", "tap state idle --before-some-flag")
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if hooksCurrent(path, testHooks) {
		t.Errorf("changed commands should not be current")
	}

	// Missing file: freshness only means anything once hooks are installed,
	// so it stays current and "not installed" remains the only finding.
	if !hooksCurrent(filepath.Join(t.TempDir(), "nope.json"), testHooks) {
		t.Errorf("missing file should count as current")
	}
}

func TestHooksCurrentIgnoresForeignHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installHooks(path, testHooks); err != nil {
		t.Fatal(err)
	}
	if !hooksCurrent(path, testHooks) {
		t.Errorf("foreign hooks must not affect freshness")
	}
}

func TestPiCurrent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	if !piCurrent() {
		t.Errorf("missing extension should count as current")
	}
	if err := installPi(); err != nil {
		t.Fatal(err)
	}
	if !piCurrent() {
		t.Errorf("fresh install should be current")
	}
	path := filepath.Join(dir, "extensions", "tap-agent-state.ts")
	if err := os.WriteFile(path, []byte(piMarker+"\n// older version"), 0o644); err != nil {
		t.Fatal(err)
	}
	if piCurrent() {
		t.Errorf("diverged extension should not be current")
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
	if got := backups(t, path); len(got) != 0 {
		t.Fatalf("fresh install created backups: %q", got)
	}

	if err := uninstallPi(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("extension should be removed by uninstall")
	}
	if got := backups(t, path); !slices.Equal(got, []string{string(piExtension)}) {
		t.Errorf("uninstall backup = %q, want installed extension", got)
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
	if got := backups(t, path); len(got) != 0 {
		t.Errorf("unmodified foreign file created backups: %q", got)
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
