package agents

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func putPiSettings(t *testing.T, dir, source string, command []string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"packages": []string{source}, "npmCommand": command})
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "settings.json"), data)
}

func putPiPackage(t *testing.T, root string) {
	t.Helper()
	put(t, filepath.Join(root, "package.json"), []byte(`{"name":"pi-tmux-agent-panel","version":"0.1.12"}`))
	put(t, filepath.Join(root, "extensions", "tap-agent-state.ts"), piExtension)
}

func TestPiNativeLocalPaths(t *testing.T) {
	for _, tc := range []struct{ source, relative string }{
		{"local-tap", "local-tap"},
		{"./local-tap", "local-tap"},
		{"../local-tap", "../local-tap"},
		{"~", "."},
		{"  local-tap  ", "local-tap"},
		{"~/local-tap", "local-tap"},
		{"$DIR/local-tap", "local-tap"},
		{"file://$DIR/local-tap", "local-tap"},
		{"file://localhost$DIR/local-tap", "local-tap"},
		{"file://$DIR/local%20tap", "local tap"},
		{"github.com/ahmedelgabri/tmux-agent-panel", "github.com/ahmedelgabri/tmux-agent-panel"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "agent")
			t.Setenv("PI_CODING_AGENT_DIR", dir)
			t.Setenv("HOME", dir)
			t.Chdir(t.TempDir())
			root := filepath.Join(dir, tc.relative)
			putPiPackage(t, root)
			base := dir
			if strings.HasPrefix(tc.source, "file://") {
				base = (&url.URL{Path: dir}).EscapedPath()
			}
			putPiSettings(t, dir, strings.ReplaceAll(tc.source, "$DIR", base), nil)
			if detail, err := piNative(); err != nil || !strings.Contains(detail, root) {
				t.Fatalf("local package: %q, %v", detail, err)
			}
			put(t, filepath.Join(root, "extensions", "tap-agent-state.ts"), []byte("stale"))
			if _, err := piNative(); err == nil || !strings.Contains(err.Error(), "outdated") {
				t.Fatalf("stale local package: %v", err)
			}
		})
	}
}

func TestPiLocalFileURLValidation(t *testing.T) {
	for _, source := range []string{"file://remote-host/tmp/tap", "file:///tmp/tap%2fextension", "file:///tmp/bad%escape"} {
		if _, err := piLocalPath(t.TempDir(), source); err == nil {
			t.Errorf("invalid file URL accepted: %q", source)
		}
	}
}

func TestPiNativeUnavailableNpmPaths(t *testing.T) {
	for _, tc := range []struct{ name, manager, script string }{
		{"command absent", "npm", ""},
		{"command failure", "npm", "exit 1"},
		{"empty root", "npm", "exit 0"},
		{"missing root", "npm", `printf '%s/missing\n' "$TAP_TEST_NPM_ROOT"`},
		{"pnpm failure", "pnpm", "exit 1"},
		{"pnpm invalid JSON", "pnpm", `printf '%s\n' '{'`},
		{"pnpm null JSON", "pnpm", `printf '%s\n' 'null'`},
		{"pnpm missing path", "pnpm", `printf '[{"dependencies":{"pi-tmux-agent-panel":{"path":"%s/missing"}}}]\n' "$TAP_TEST_NPM_ROOT"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "agent")
			bin := filepath.Join(base, "bin")
			global := filepath.Join(base, "global", "node_modules")
			t.Setenv("PI_CODING_AGENT_DIR", dir)
			t.Setenv("PATH", bin)
			t.Setenv("TAP_TEST_NPM_ROOT", global)
			putPiSettings(t, dir, "npm:pi-tmux-agent-panel", []string{tc.manager})
			putPiPackage(t, filepath.Join(global, "pi-tmux-agent-panel"))
			if tc.script != "" {
				script := "#!/bin/sh\n" + tc.script + "\n"
				if tc.manager == "pnpm" {
					// An unusable pnpm result must not trigger a root lookup.
					script = "#!/bin/sh\ncase \"$*\" in\n'list -g --depth 0 --json') " + tc.script + ";;\n'root -g') printf '%s\\n' \"$TAP_TEST_NPM_ROOT\";;\nesac\n"
				}
				executable := filepath.Join(bin, tc.manager)
				put(t, executable, []byte(script))
				if err := os.Chmod(executable, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			managed := filepath.Join(dir, "npm", "node_modules", "pi-tmux-agent-panel")
			if detail, err := piNative(); detail != "" || err == nil || !strings.Contains(err.Error(), managed) {
				t.Fatalf("unavailable package: %q, %v", detail, err)
			}
		})
	}
}

// Only read-only lookup commands are supported by this fake package manager.
const piNpmScript = `#!/bin/sh
printf '%s\n' "$*" >> "$TAP_TEST_NPM_LOG"
case "$*" in
  *'list -g --depth 0 --json') printf '%s\n' "$TAP_TEST_PNPM_LIST" ;;
  *'root -g') printf '%s\n' "$TAP_TEST_NPM_ROOT" ;;
  *'pm bin -g') printf '%s\n' "$TAP_TEST_BUN_BIN" ;;
  *) exit 1 ;;
esac
`

func TestPiNativeLegacyNpmPaths(t *testing.T) {
	for _, tc := range []struct {
		name, manager string
		command       []string
		pnpmPath      bool
	}{
		{"default npm", "npm", nil, false},
		{"configured npm", "npm", []string{"$BIN/npm", "--userconfig", "config with spaces"}, false},
		{"pnpm dependency path", "pnpm", []string{"pnpm"}, true},
		{"wrapped pnpm", "pnpm", []string{"mise", "exec", "node@20", "--", "pnpm"}, true},
		{"pnpm root fallback", "pnpm", []string{"pnpm"}, false},
		{"bun global path", "bun", []string{"bun"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "agent")
			bin := filepath.Join(base, "bin with spaces")
			log := filepath.Join(base, "commands.log")
			t.Setenv("PI_CODING_AGENT_DIR", dir)
			t.Setenv("PATH", bin)
			t.Setenv("TAP_TEST_NPM_LOG", log)
			t.Setenv("TAP_TEST_NPM_ROOT", filepath.Join(base, "global", "node_modules"))
			t.Setenv("TAP_TEST_BUN_BIN", filepath.Join(base, "bun", "bin"))
			legacy := filepath.Join(base, "global", "node_modules", "pi-tmux-agent-panel")
			if tc.pnpmPath {
				legacy = filepath.Join(base, "pnpm-store", "package")
			} else if tc.manager == "bun" {
				legacy = filepath.Join(base, "bun", "install", "global", "node_modules", "pi-tmux-agent-panel")
			}
			listing := "[]"
			if tc.pnpmPath {
				data, err := json.Marshal([]any{map[string]any{"dependencies": map[string]any{"pi-tmux-agent-panel": map[string]string{"path": legacy}}}})
				if err != nil {
					t.Fatal(err)
				}
				listing = string(data)
			}
			t.Setenv("TAP_TEST_PNPM_LIST", listing)
			command := append([]string(nil), tc.command...)
			for i := range command {
				command[i] = strings.ReplaceAll(command[i], "$BIN", bin)
			}
			executable := filepath.Join(bin, "npm")
			if len(command) > 0 {
				executable = filepath.Join(bin, filepath.Base(command[0]))
			}
			put(t, executable, []byte(piNpmScript))
			if err := os.Chmod(executable, 0o755); err != nil {
				t.Fatal(err)
			}
			putPiSettings(t, dir, "npm:pi-tmux-agent-panel@0.1.12", command)
			putPiPackage(t, legacy)
			if detail, err := piNative(); err != nil || !strings.Contains(detail, legacy) {
				t.Fatalf("legacy package: %q, %v", detail, err)
			}
			prefix := ""
			if len(command) > 1 {
				prefix = strings.Join(command[1:], " ") + " "
			}
			wantCalls := prefix + "root -g\n"
			if tc.manager == "pnpm" {
				wantCalls = prefix + "list -g --depth 0 --json\n"
				if !tc.pnpmPath {
					wantCalls += prefix + "root -g\n"
				}
			} else if tc.manager == "bun" {
				wantCalls = prefix + "pm bin -g\n"
			}
			if calls, err := os.ReadFile(log); err != nil || string(calls) != wantCalls {
				t.Fatalf("lookup commands: %q, %v; want %q", calls, err, wantCalls)
			}

			// Existence, not freshness, determines precedence over legacy installs.
			managed := filepath.Join(dir, "npm", "node_modules", "pi-tmux-agent-panel")
			if err := os.MkdirAll(managed, 0o755); err != nil {
				t.Fatal(err)
			}
			put(t, log, nil)
			if _, err := piNative(); err == nil || !strings.Contains(err.Error(), managed) {
				t.Fatalf("incomplete managed package must win: %v", err)
			}
			putPiPackage(t, managed)
			if detail, err := piNative(); err != nil || !strings.Contains(detail, managed) {
				t.Fatalf("managed package: %q, %v", detail, err)
			}
			put(t, filepath.Join(managed, "extensions", "tap-agent-state.ts"), []byte("stale"))
			if _, err := piNative(); err == nil || !strings.Contains(err.Error(), "outdated") {
				t.Fatalf("stale managed package must not use healthy legacy: %v", err)
			}
			if calls, err := os.ReadFile(log); err != nil || len(calls) != 0 {
				t.Fatalf("managed package must bypass npm commands: %q, %v", calls, err)
			}
		})
	}
}
