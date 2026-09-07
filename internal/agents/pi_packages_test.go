package agents

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiNativePackageDeduplication(t *testing.T) {
	const git = "git:github.com/ahmedelgabri/tmux-agent-panel"
	const npm = "npm:pi-tmux-agent-panel"
	for _, tc := range []struct {
		name, first, second string
		sameIdentity        bool
	}{
		{"npm versions", npm + "@0.1.12", npm + "@0.2.0", true},
		{"npm unpinned", npm, npm + "@0.1.12", true},
		{"npm whitespace", "npm: pi-tmux-agent-panel@^0.1.0 ", npm, true},
		{"git refs", git + "@v0.1.12", git + "@main", true},
		{"git transports", "git:git@github.com:ahmedelgabri/tmux-agent-panel.git@main", "https://github.com/ahmedelgabri/tmux-agent-panel", true},
		{"git SSH", "ssh://git@github.com/ahmedelgabri/tmux-agent-panel.git", git, true},
		{"local relative", "local-tap", "./local-tap", true},
		{"local normalized", "local-tap", "unused/../local-tap", true},
		{"local absolute", "local-tap", "$DIR/local-tap", true},
		{"local home", "~/local-tap", "local-tap", true},
		{"local file URL", "$URL/local-tap", "local-tap", true},
		{"local encoded URL", "$URL/local%20tap", "local tap", true},
		{"distinct local copies", "local-tap", "other-tap", false},
		{"distinct source kinds", git, npm, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "agent")
			t.Setenv("PI_CODING_AGENT_DIR", dir)
			t.Setenv("HOME", dir)
			t.Setenv("PATH", t.TempDir())
			t.Chdir(t.TempDir())
			roots := []string{
				filepath.Join(dir, "local-tap"),
				filepath.Join(dir, "local tap"),
				filepath.Join(dir, "other-tap"),
				filepath.Join(dir, "git", "github.com", "ahmedelgabri", "tmux-agent-panel"),
				filepath.Join(dir, "npm", "node_modules", "pi-tmux-agent-panel"),
			}
			for _, root := range roots {
				putPiPackage(t, root)
			}
			replacer := strings.NewReplacer("$DIR", dir, "$URL", (&url.URL{Scheme: "file", Path: dir}).String())
			first, second := replacer.Replace(tc.first), replacer.Replace(tc.second)
			check := func(entries []any, want bool) {
				t.Helper()
				settings, err := json.Marshal(map[string]any{"packages": entries})
				if err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(dir, "settings.json"), settings)
				if detail, err := piNative(); err != nil || (detail != "") != want {
					t.Fatalf("packages=%s: %q, %v; enabled=%v", settings, detail, err, want)
				}
			}
			for _, filter := range []map[string]any{
				{"autoload": false},
				{"extensions": []string{}},
				{"extensions": []string{"!**/*.ts"}},
			} {
				filter["source"] = first
				check([]any{filter, second}, !tc.sameIdentity)
				check([]any{filter, map[string]any{"source": second, "autoload": false, "extensions": []string{"+extensions/tap-agent-state.ts"}}}, !tc.sameIdentity)
			}
			check([]any{first, map[string]any{"source": second, "extensions": []string{}}}, true)
			if tc.sameIdentity {
				check([]any{map[string]any{"source": first, "autoload": false}, map[string]any{"source": second, "autoload": "ignored", "extensions": "ignored"}}, false)
				for _, root := range roots {
					if err := os.RemoveAll(root); err != nil {
						t.Fatal(err)
					}
				}
				// A skipped duplicate must not trigger missing-package diagnostics.
				check([]any{map[string]any{"source": first, "autoload": false}, second}, false)
			}
		})
	}
}

func TestPiNativeInvalidNpmNameDoesNotShadowPackage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	t.Setenv("PATH", t.TempDir())
	putPiPackage(t, filepath.Join(dir, "npm", "node_modules", "pi-tmux-agent-panel"))
	put(t, filepath.Join(dir, "settings.json"), []byte(`{"packages":[{"source":"npm:pi-tmux-agent-panel@","autoload":false},"npm:pi-tmux-agent-panel"]}`))
	if detail, err := piNative(); err != nil || detail == "" {
		t.Fatalf("invalid npm name shadowed tap: %q, %v", detail, err)
	}
}

func TestDoctorPiDisabledDuplicate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	bin := t.TempDir()
	put(t, filepath.Join(bin, "pi"), []byte("#!/bin/sh\nexit 0\n"))
	if err := os.Chmod(filepath.Join(bin, "pi"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	putPiPackage(t, filepath.Join(dir, "local-tap"))
	put(t, filepath.Join(dir, "settings.json"), []byte(`{"packages":[{"source":"local-tap","extensions":[]},"./local-tap"]}`))
	for _, direct := range []bool{false, true} {
		if direct {
			put(t, filepath.Join(dir, "extensions", "tap-agent-state.ts"), piExtension)
		}
		found := false
		for _, check := range Doctor() {
			if check.Name != "pi" {
				continue
			}
			found = true
			if check.OK != direct || strings.Contains(check.Detail, "native package installed") || strings.Contains(check.Detail, "both install channels") {
				t.Fatalf("direct=%v: disabled duplicate misreported: %+v", direct, check)
			}
		}
		if !found {
			t.Fatal("pi check missing")
		}
	}
}
