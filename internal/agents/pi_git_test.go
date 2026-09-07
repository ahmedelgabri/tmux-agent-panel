package agents

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiGitSource(t *testing.T) {
	for _, tc := range piGitCases {
		t.Run(tc.source, func(t *testing.T) {
			host, repo := parsePiGitSource(tc.source)
			if host != tc.host || repo != tc.repo {
				t.Fatalf("identity=%q/%q, want %q/%q", host, repo, tc.host, tc.repo)
			}
		})
	}
}

func TestPiNativeGitSources(t *testing.T) {
	for _, tc := range piGitCases {
		t.Run(tc.source, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PI_CODING_AGENT_DIR", dir)
			t.Setenv("HOME", dir)
			t.Setenv("PATH", t.TempDir())
			canonicalRepo := "ahmedelgabri/tmux-agent-panel"
			root := filepath.Join(dir, "git", "github.com", canonicalRepo)
			putPiPackage(t, root)
			want := strings.EqualFold(tc.host, "github.com") && strings.EqualFold(filepath.ToSlash(filepath.Clean(tc.repo)), canonicalRepo)
			if want {
				root = filepath.Join(dir, "git", tc.host, tc.repo)
				putPiPackage(t, root)
			}
			check := func(entries []any, want bool) {
				t.Helper()
				settings, err := json.Marshal(map[string]any{"packages": entries})
				if err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(dir, "settings.json"), settings)
				if detail, err := piNative(); err != nil || (detail != "") != want {
					t.Fatalf("settings=%s: %q, %v; want enabled=%v", settings, detail, err, want)
				}
			}
			check([]any{tc.source}, want)
			// An alias must participate in first-entry deduplication, but Pi keeps
			// identities with different host/path spelling separate.
			duplicate := tc.host == "github.com" && tc.repo == canonicalRepo
			check([]any{map[string]any{"source": tc.source, "autoload": false}, "git:github.com/" + canonicalRepo}, !duplicate)
			if want {
				put(t, filepath.Join(root, "extensions", "tap-agent-state.ts"), []byte("stale"))
				settings, err := json.Marshal(map[string]any{"packages": []string{tc.source}})
				if err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(dir, "settings.json"), settings)
				if detail, err := piNative(); detail != "" || err == nil || !strings.Contains(err.Error(), "outdated") {
					t.Fatalf("stale Git package: %q, %v", detail, err)
				}
				if err := os.Remove(filepath.Join(root, "extensions", "tap-agent-state.ts")); err != nil {
					t.Fatal(err)
				}
				if detail, err := piNative(); detail != "" || !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("missing Git package: %q, %v", detail, err)
				}
			}
		})
	}
}
