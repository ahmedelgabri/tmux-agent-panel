package agents

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestPiNativePackageFilters(t *testing.T) {
	for _, tc := range []struct {
		name     string
		filters  []string
		autoload bool
		want     bool
	}{
		{"ordered exact include", []string{"-extensions/tap-agent-state.ts", "+extensions/tap-agent-state.ts"}, false, true},
		{"ordered exact exclude", []string{"+extensions/tap-agent-state.ts", "-extensions/tap-agent-state.ts"}, false, false},
		{"ordered glob include", []string{"!**/*.ts", "extensions/**/tap-agent-state.ts"}, false, true},
		{"ordered glob exclude", []string{"+extensions/tap-agent-state.ts", "!extensions/**/*.ts"}, false, false},
		{"absolute glob", []string{"$ROOT/**/*.ts"}, false, true},
		{"absolute exact", []string{"+$ROOT/extensions/tap-agent-state.ts"}, false, true},
		{"normal exact precedence", []string{"-extensions/tap-agent-state.ts", "+extensions/tap-agent-state.ts"}, true, false},
		{"normal force include", []string{"+extensions/tap-agent-state.ts", "!**/*.ts"}, true, true},
		{"empty normal filter", []string{}, true, false},
		{"no delta", nil, false, false},
		{"unfiltered", nil, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			root := filepath.Join(dir, "local-tap")
			t.Setenv("PI_CODING_AGENT_DIR", dir)
			putPiPackage(t, root)
			pkg := map[string]any{"source": "local-tap", "autoload": tc.autoload}
			if tc.filters != nil {
				filters := make([]string, len(tc.filters))
				for i, filter := range tc.filters {
					filters[i] = strings.ReplaceAll(filter, "$ROOT", filepath.ToSlash(root))
				}
				pkg["extensions"] = filters
			}
			settings, err := json.Marshal(map[string]any{"packages": []any{pkg}})
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(dir, "settings.json"), settings)
			if detail, err := piNative(); err != nil || (detail != "") != tc.want {
				t.Fatalf("filtered package: %q, %v; enabled=%v", detail, err, tc.want)
			}
			put(t, filepath.Join(root, piPackageExtension), []byte("stale"))
			if _, err := piNative(); (err != nil) != tc.want {
				t.Fatalf("stale filtered package: %v; enabled=%v", err, tc.want)
			}
		})
	}
}

func TestPiFilterGlobstarDotDirectory(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		want    bool
	}{
		{"/packages/*/extensions/*.ts", false},
		{"/packages/**/extensions/*.ts", false},
		{"/packages/.*/extensions/*.ts", true},
		{"/packages/.tap/**/*.ts", true},
	} {
		if got := piExtensionEnabled("/packages/.tap", []string{tc.pattern}, false); got != tc.want {
			t.Errorf("pattern %q: got %v, want %v", tc.pattern, got, tc.want)
		}
	}
}
