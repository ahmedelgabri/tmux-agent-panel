package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedelgabri/tmux-agent-panel/plugins"
)

func put(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeNative(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	settings := filepath.Join(dir, "settings.json")
	put(t, settings, []byte(`{"enabledPlugins":{"tap@tmux-agent-panel":true}}`))
	if _, err := claudeNative(); err == nil {
		t.Fatal("enabled but missing plugin must fail")
	}
	root := filepath.Join(dir, "plugins", "cache", "tmux-agent-panel", "tap", "0.1.12")
	registry, err := json.Marshal(map[string]any{"version": 2, "plugins": map[string]any{
		"tap@tmux-agent-panel": []any{map[string]string{"scope": "user", "installPath": root}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "plugins", "installed_plugins.json"), registry)
	put(t, filepath.Join(root, "hooks", "hooks.json"), plugins.ClaudeHooks)
	if detail, err := claudeNative(); err != nil || !strings.Contains(detail, root) {
		t.Fatalf("installed native plugin: %q, %v", detail, err)
	}
	put(t, settings, []byte(`{"enabledPlugins":{"tap@tmux-agent-panel":false}}`))
	if detail, err := claudeNative(); err != nil || detail != "" {
		t.Fatalf("disabled plugin must not count: %q, %v", detail, err)
	}
}

func TestCodexNative(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	config := filepath.Join(dir, "config.toml")
	put(t, config, []byte("[plugins.\"tap-codex@tmux-agent-panel\"]\nenabled = true\n"))
	if _, err := codexNative(); err == nil {
		t.Fatal("enabled but missing plugin must fail")
	}
	base := filepath.Join(dir, "plugins", "cache", "tmux-agent-panel", "tap-codex")
	put(t, filepath.Join(base, "0.1.9", "hooks", "hooks.json"), testHooks)
	put(t, filepath.Join(base, "0.1.12", "hooks", "hooks.json"), plugins.CodexHooks)
	if detail, err := codexNative(); err != nil || !strings.Contains(detail, "0.1.12") {
		t.Fatalf("highest version must win: %q, %v", detail, err)
	}
	put(t, filepath.Join(base, "local", "hooks", "hooks.json"), testHooks)
	if _, err := codexNative(); err == nil || !strings.Contains(err.Error(), "outdated") {
		t.Fatalf("local must take precedence: %v", err)
	}
	put(t, config, []byte("[plugins.\"tap-codex@tmux-agent-panel\"]\nenabled = false\n"))
	if detail, err := codexNative(); err != nil || detail != "" {
		t.Fatalf("disabled plugin must not count: %q, %v", detail, err)
	}
}

func TestCodexNativeEnablement(t *testing.T) {
	const table = "[plugins.\"tap-codex@tmux-agent-panel\"]\n"
	for _, tc := range []struct {
		name, config string
		enabled      bool
	}{
		{"absent table", "", false},
		{"unrelated table", "[plugins.\"other@marketplace\"]\n", false},
		{"empty table", table, true},
		{"explicit true", table + "enabled = true\n", true},
		{"explicit false", table + "enabled = false\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("CODEX_HOME", dir)
			put(t, filepath.Join(dir, "config.toml"), []byte(tc.config))
			base := filepath.Join(dir, "plugins", "cache", "tmux-agent-panel", "tap-codex")
			root := filepath.Join(base, "local")
			put(t, filepath.Join(root, "hooks", "hooks.json"), plugins.CodexHooks)
			if detail, err := codexNative(); err != nil || (detail != "") != tc.enabled {
				t.Fatalf("cached plugin: %q, %v; enabled=%v", detail, err, tc.enabled)
			}
			if err := os.RemoveAll(base); err != nil {
				t.Fatal(err)
			}
			if detail, err := codexNative(); (err != nil) != tc.enabled || detail != "" {
				t.Fatalf("missing plugin: %q, %v; enabled=%v", detail, err, tc.enabled)
			}
		})
	}
}

func TestCodexNativePluginFeature(t *testing.T) {
	const table = "[plugins.\"tap-codex@tmux-agent-panel\"]\n"
	const disabled = "[features]\nplugins = false\n"
	for _, tc := range []struct {
		name, config  string
		active, gated bool
	}{
		{"omitted feature", table, true, false},
		{"empty features", "[features]\n" + table, true, false},
		{"enabled feature", "[features]\nplugins = true\n" + table, true, false},
		{"disabled feature with default enablement", disabled + table, false, true},
		{"disabled feature with explicit enablement", disabled + table + "enabled = true\n", false, true},
		{"disabled plugin", disabled + table + "enabled = false\n", false, false},
		{"absent plugin", disabled, false, false},
		{"unrelated plugin", disabled + "[plugins.\"other@marketplace\"]\n", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("CODEX_HOME", dir)
			config := filepath.Join(dir, "config.toml")
			put(t, config, []byte(tc.config))
			root := filepath.Join(dir, "plugins", "cache", "tmux-agent-panel", "tap-codex", "local")
			put(t, filepath.Join(root, "hooks", "hooks.json"), plugins.CodexHooks)
			for _, cached := range []bool{true, false} {
				if !cached {
					if err := os.RemoveAll(filepath.Join(dir, "plugins")); err != nil {
						t.Fatal(err)
					}
				}
				detail, err := codexNative()
				if (detail != "") != (tc.active && cached) {
					t.Fatalf("cached=%v: %q, %v", cached, detail, err)
				}
				if tc.gated {
					if err == nil || !strings.Contains(err.Error(), "features.plugins = false") || !strings.Contains(err.Error(), config) {
						t.Fatalf("cached=%v: disabled feature must identify its config: %v", cached, err)
					}
				} else if (err != nil) != (tc.active && !cached) {
					t.Fatalf("cached=%v: unexpected error: %v", cached, err)
				}
			}
		})
	}
}

func TestDoctorCodexPluginFeature(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir())
	bin := t.TempDir()
	put(t, filepath.Join(bin, "codex"), []byte("#!/bin/sh\nexit 0\n"))
	if err := os.Chmod(filepath.Join(bin, "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	for _, tc := range []struct {
		name, plugin string
		direct, ok   bool
	}{
		{"native only", "[plugins.\"tap-codex@tmux-agent-panel\"]\n", false, false},
		{"direct and gated native", "[plugins.\"tap-codex@tmux-agent-panel\"]\n", true, false},
		{"direct only", "", true, true},
		{"direct with disabled plugin", "[plugins.\"tap-codex@tmux-agent-panel\"]\nenabled = false\n", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("CODEX_HOME", dir)
			put(t, filepath.Join(dir, "config.toml"), []byte("[features]\nplugins = false\n"+tc.plugin))
			put(t, filepath.Join(dir, "plugins", "cache", "tmux-agent-panel", "tap-codex", "local", "hooks", "hooks.json"), plugins.CodexHooks)
			if tc.direct {
				put(t, filepath.Join(dir, "hooks.json"), plugins.CodexHooks)
			}
			for _, check := range Doctor() {
				if check.Name != "codex" {
					continue
				}
				if check.OK != tc.ok || strings.Contains(check.Detail, "features.plugins = false") != !tc.ok || strings.Contains(check.Detail, "native plugin installed") {
					t.Fatalf("incorrect feature diagnostic: %+v", check)
				}
				return
			}
			t.Fatal("codex check missing")
		})
	}
}

func TestPiNative(t *testing.T) {
	for _, source := range []string{
		"https://github.com/ahmedelgabri/tmux-agent-panel",
		"git:github.com/ahmedelgabri/tmux-agent-panel@v0.1.12",
		"git:git@github.com:ahmedelgabri/tmux-agent-panel.git@main",
		"ssh://git@github.com/ahmedelgabri/tmux-agent-panel",
		"npm:pi-tmux-agent-panel@0.1.12",
		"./local-tap",
	} {
		t.Run(source, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PI_CODING_AGENT_DIR", dir)
			root := filepath.Join(dir, "git", "github.com", "ahmedelgabri", "tmux-agent-panel")
			if strings.HasPrefix(source, "npm:") {
				root = filepath.Join(dir, "npm", "node_modules", "pi-tmux-agent-panel")
			} else if strings.HasPrefix(source, "./") {
				root = filepath.Join(dir, "local-tap")
			}
			put(t, filepath.Join(root, "package.json"), []byte(`{"name":"pi-tmux-agent-panel"}`))
			put(t, filepath.Join(root, "extensions", "tap-agent-state.ts"), piExtension)
			settings, err := json.Marshal(map[string]any{"packages": []string{source}})
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(dir, "settings.json"), settings)
			if detail, err := piNative(); err != nil || !strings.Contains(detail, root) {
				t.Fatalf("native pi package: %q, %v", detail, err)
			}
			put(t, filepath.Join(root, "extensions", "tap-agent-state.ts"), []byte("// stale"))
			if _, err := piNative(); err == nil || !strings.Contains(err.Error(), "outdated") {
				t.Fatalf("stale package: %v", err)
			}
		})
	}
}

func TestPiNativeDisabledAndUnrelated(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	t.Setenv("PATH", t.TempDir())
	for _, settings := range []string{
		`{"packages":[{"source":"https://github.com/ahmedelgabri/tmux-agent-panel","extensions":[]}]}`,
		`{"packages":[{"source":"https://github.com/ahmedelgabri/tmux-agent-panel","autoload":false}]}`,
		`{"packages":["https://github.com/other/tmux-agent-panel"]}`,
		`{"packages":["https://github.com/ahmedelgabri/tmux-agent-panel-fork"]}`,
	} {
		put(t, filepath.Join(dir, "settings.json"), []byte(settings))
		if detail, err := piNative(); err != nil || detail != "" {
			t.Fatalf("disabled or foreign package: %q, %v", detail, err)
		}
	}
	put(t, filepath.Join(dir, "settings.json"), []byte(`{"packages":["npm:pi-tmux-agent-panel"]}`))
	if _, err := piNative(); err == nil {
		t.Fatal("configured but missing package must fail")
	}
}

func TestPiExtensionFilters(t *testing.T) {
	for _, tc := range []struct {
		filters  []string
		autoload bool
		want     bool
	}{
		{nil, true, true},
		{nil, false, false},
		{[]string{}, true, false},
		{[]string{"extensions/*.ts"}, true, true},
		{[]string{"*.ts"}, true, true},
		{[]string{"!extensions/other.ts"}, true, true},
		{[]string{"!extensions/*.ts"}, true, false},
		{[]string{"!**/*.ts", "+extensions/tap-agent-state.ts"}, true, true},
		{[]string{"-extensions/tap-agent-state.ts", "+extensions/tap-agent-state.ts"}, true, false},
		{[]string{"+extensions/tap-agent-state.ts"}, false, true},
		{[]string{"extensions/*.ts"}, false, true},
		{[]string{"!extensions/other.ts"}, false, false},
		{[]string{"-extensions/tap-agent-state.ts", "+extensions/tap-agent-state.ts"}, false, true},
		{[]string{"+extensions/tap-agent-state.ts", "-extensions/tap-agent-state.ts"}, false, false},
		{[]string{"+extensions/tap-agent-state.ts", "!**/*.ts"}, false, false},
		{[]string{"!**/*.ts", "+extensions/tap-agent-state.ts"}, false, true},
		{[]string{"-extensions/tap-agent-state.ts", "extensions/**/*.ts"}, false, true},
		{[]string{"extensions/**/*.ts", "!**/*.ts", "**/{tap,other}-agent-state.ts"}, false, true},
		{[]string{"extensions/**/*.ts", "other/**"}, false, true},
		{[]string{"extensions/**/*.ts", "-extensions/*.ts"}, false, true},
		{[]string{"+tap-agent-state.ts"}, false, false},
		{[]string{"+./extensions/tap-agent-state.ts"}, false, true},
		{[]string{"extensions/**/tap-agent-state.ts"}, false, true},
		{[]string{"**/extensions/**/tap-agent-state.ts"}, true, true},
		{[]string{"extensions/**/**/tap-agent-state.ts"}, true, true},
		{[]string{"extensions/*/tap-agent-state.ts"}, true, false},
		{[]string{"extensions/**/*.{ts,js}"}, true, true},
		{[]string{"{extensions,other}/tap-agent-state.ts"}, true, true},
		{[]string{"extensions/+(tap|other)-agent-state.ts"}, true, true},
		{[]string{"extensions/!(other).ts"}, true, true},
		{[]string{"extensions/[!x]ap-agent-state.ts"}, true, true},
		{[]string{"./extensions/*.ts"}, true, false},
		{[]string{"/packages/tap/**/*.ts"}, false, true},
		{[]string{"+/packages/tap/extensions/tap-agent-state.ts"}, false, true},
		{[]string{"/packages/tap/**/*.ts", "-/packages/tap/extensions/tap-agent-state.ts"}, false, false},
		{[]string{"/packages/other/**/*.ts"}, true, false},
		{[]string{"+extensions/tap-agent-state.ts", "!**/*.ts"}, true, true},
		{[]string{"extensions/other.ts"}, true, false},
		{[]string{"extensions/other.ts", "*.ts"}, true, true},
		{[]string{"*.ts", "!extensions/*.ts"}, true, false},
		{[]string{"+extensions/other.ts", "!*.ts"}, true, false},
		{[]string{"+extensions/tap-agent-state.ts", "-extensions/tap-agent-state.ts"}, true, false},
		{[]string{"-extensions/*.ts"}, true, true},
		{[]string{"[invalid"}, true, false},
		{[]string{""}, true, false},
	} {
		if got := piExtensionEnabled("/packages/tap", tc.filters, tc.autoload); got != tc.want {
			t.Errorf("filters %v, autoload %v: got %v, want %v", tc.filters, tc.autoload, got, tc.want)
		}
	}
}

func TestDoctorPiWiring(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	t.Setenv("CODEX_HOME", filepath.Join(dir, "codex"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(dir, "pi"))
	bin := filepath.Join(dir, "bin")
	for _, name := range []string{"tmux", "tap", "pi"} {
		file := filepath.Join(bin, name)
		put(t, file, []byte("#!/bin/sh\nexit 0\n"))
		if err := os.Chmod(file, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	for _, tc := range []struct {
		direct, native string
		detected, ok   bool
	}{
		{"absent", "current", true, true},
		{"current", "absent", true, true},
		{"current", "current", true, true},
		{"absent", "absent", true, false},
		{"stale", "current", true, false},
		{"stale", "absent", true, false},
		{"stale", "missing", true, false},
		{"current", "stale", true, false},
		{"current", "missing", true, false},
		{"absent", "absent", false, true},
		{"stale", "stale", false, true},
	} {
		t.Run(fmt.Sprintf("%s/%s/detected=%v", tc.direct, tc.native, tc.detected), func(t *testing.T) {
			piDir := t.TempDir()
			t.Setenv("PI_CODING_AGENT_DIR", piDir)
			if !tc.detected {
				t.Setenv("PATH", t.TempDir())
			}
			for path, status := range map[string]string{
				filepath.Join(piDir, "extensions", "tap-agent-state.ts"):                                               tc.direct,
				filepath.Join(piDir, "npm", "node_modules", "pi-tmux-agent-panel", "extensions", "tap-agent-state.ts"): tc.native,
			} {
				switch status {
				case "current":
					put(t, path, piExtension)
				case "stale":
					put(t, path, []byte(piMarker))
				}
			}
			if tc.native != "absent" {
				put(t, filepath.Join(piDir, "settings.json"), []byte(`{"packages":["npm:pi-tmux-agent-panel"]}`))
			}
			for _, check := range Doctor() {
				if check.Name != "pi" {
					continue
				}
				if check.OK != tc.ok {
					t.Errorf("unexpected status: %+v", check)
				}
				if tc.native == "current" && (!strings.Contains(check.Detail, "native package installed") || strings.Contains(check.Detail, "not installed")) {
					t.Errorf("native install misreported: %+v", check)
				}
				if tc.direct != "absent" && tc.native == "current" && !strings.Contains(check.Detail, "both install channels") {
					t.Errorf("duplicate install not reported: %+v", check)
				}
				if tc.direct == "stale" {
					want, unwanted := "tap install", "tap uninstall"
					if tc.native == "current" {
						want, unwanted = "tap uninstall --pi", "tap install"
					}
					if !strings.Contains(check.Detail, want) || strings.Contains(check.Detail, unwanted) {
						t.Errorf("incorrect recovery advice: %+v", check)
					}
				}
				return
			}
			t.Fatal("pi check missing")
		})
	}
}
