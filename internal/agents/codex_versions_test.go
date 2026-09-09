package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedelgabri/tmux-agent-panel/plugins"
)

func TestValidCodexPluginVersion(t *testing.T) {
	for _, version := range []string{"local", "0.2.0", "1.0.0-rc.1+build.2", "snapshot_1", "ABC-_.+09", "..."} {
		if !validCodexPluginVersion(version) {
			t.Errorf("valid cache segment rejected: %q", version)
		}
	}
	for _, version := range []string{"", ".", "..", "zzz!", "0.2.0 ", "a/b", `a\b`, "é", "v\n1", "\xff"} {
		if validCodexPluginVersion(version) {
			t.Errorf("invalid cache segment accepted: %q", version)
		}
	}
}

func TestCompareCodexPluginVersions(t *testing.T) {
	for _, pair := range [][2]string{
		{"0.2.9", "0.2.10"},
		{"0.2.0-alpha", "0.2.0-alpha.1"},
		{"0.2.0-alpha.1", "0.2.0-alpha.beta"},
		{"0.2.0-beta.2", "0.2.0-beta.11"},
		{"0.2.0-rc.1", "0.2.0"},
		{"0.9.0", "0.10.0-rc.1"},
		{"0.2.0-18446744073709551616", "0.2.0-18446744073709551617"},
		{"9.0.0", "18446744073709551615.0.0"},
		{"0.10", "0.9.0"},
		{"0.10.0-01", "0.9.0"},
		{"18446744073709551616.0.0", "9.0.0"},
		{"0.2.0", "v0.1.0"},
		{"0.2.0", "snapshot_1"},
	} {
		assertCodexVersionOrder(t, pair[0], pair[1])
	}
	builds := []string{"", "+0", "+00", "+1", "+01", "+001", "+2", "+02", "+002", "+10", "+a", "+a.1", "+a.2", "+a.10"}
	for i := 1; i < len(builds); i++ {
		assertCodexVersionOrder(t, "0.2.0"+builds[i-1], "0.2.0"+builds[i])
	}
}

func assertCodexVersionOrder(t *testing.T, left, right string) {
	t.Helper()
	if compareCodexPluginVersions(left, right) >= 0 || compareCodexPluginVersions(right, left) <= 0 || compareCodexPluginVersions(left, left) != 0 {
		t.Errorf("expected %q < %q", left, right)
	}
}

func TestCodexNativeCacheVersions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		versions []string
		want     string
	}{
		{"invalid directory", []string{"0.2.0", "zzz!"}, "0.2.0"},
		{"invalid directories only", []string{"zzz!", "version with spaces", "é"}, ""},
		{"numeric components", []string{"0.2.9", "0.2.10"}, "0.2.10"},
		{"release over prerelease", []string{"0.2.0-rc.1", "0.2.0"}, "0.2.0"},
		{"numeric prerelease", []string{"0.2.0-rc.9", "0.2.0-rc.10"}, "0.2.0-rc.10"},
		{"prerelease core version", []string{"0.9.0", "0.10.0-rc.1"}, "0.10.0-rc.1"},
		{"build metadata", []string{"0.2.0+9", "0.2.0+10"}, "0.2.0+10"},
		{"non-semver fallback", []string{"0.2.0", "snapshot_1"}, "snapshot_1"},
		{"short version fallback", []string{"0.9.0", "0.10"}, "0.9.0"},
		{"leading zero fallback", []string{"0.9.0", "0.10.0-01"}, "0.9.0"},
		{"local precedence", []string{"0.2.0", "snapshot_1", "local", "zzz!"}, "local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("CODEX_HOME", dir)
			put(t, filepath.Join(dir, "config.toml"), []byte("[plugins.\"tap-codex@tmux-agent-panel\"]\n"))
			base := filepath.Join(dir, "plugins", "cache", "tmux-agent-panel", "tap-codex")
			for _, version := range tc.versions {
				if err := os.MkdirAll(filepath.Join(base, version), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			root := filepath.Join(base, tc.want)
			if tc.want != "" {
				put(t, filepath.Join(root, "hooks", "hooks.json"), plugins.CodexHooks)
			}
			detail, err := codexNative()
			if tc.want == "" {
				if err == nil || !strings.Contains(err.Error(), "cache empty") {
					t.Fatalf("invalid cache directories must be ignored: %q, %v", detail, err)
				}
			} else if err != nil || detail != "native plugin installed ("+root+")" {
				t.Fatalf("want cache version %q: %q, %v", tc.want, detail, err)
			}
		})
	}
}
