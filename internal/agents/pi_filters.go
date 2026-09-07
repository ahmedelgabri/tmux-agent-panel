package agents

import (
	"path/filepath"
	"strings"
	"time"

	minimatch "github.com/benjaminnkem/minimatch-go"
)

const piPackageExtension = "extensions/tap-agent-state.ts"

// Pi uses ordered deltas only when autoload is disabled; normal filters have
// fixed include/exclude precedence. Both use minimatch for non-exact patterns.
// https://github.com/earendil-works/pi/blob/v0.85.1/packages/coding-agent/src/core/package-manager.ts
func piExtensionEnabled(root string, filters []string, autoload bool) bool {
	if filters == nil {
		return autoload
	}
	if len(filters) == 0 {
		return false
	}
	full := filepath.ToSlash(filepath.Join(root, piPackageExtension))
	matches := func(pattern string) bool {
		m, err := minimatch.NewMinimatch(filepath.ToSlash(pattern), minimatch.Options{})
		if err != nil {
			return false
		}
		// Extglobs use a backtracking engine; bound pathological user patterns.
		for _, row := range m.Set {
			for _, part := range row {
				if part.MM.RE != nil {
					part.MM.RE.MatchTimeout = 250 * time.Millisecond
				}
			}
		}
		return m.Match(piPackageExtension) || m.Match(filepath.Base(piPackageExtension)) || m.Match(full)
	}
	exact := func(pattern string) bool {
		if strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, `.\`) {
			pattern = pattern[2:]
		}
		pattern = filepath.ToSlash(pattern)
		return pattern == piPackageExtension || pattern == full
	}
	if !autoload {
		enabled := false
		for _, filter := range filters {
			switch {
			case strings.HasPrefix(filter, "+"), strings.HasPrefix(filter, "-"):
				if exact(filter[1:]) {
					enabled = filter[0] == '+'
				}
			case strings.HasPrefix(filter, "!"):
				if matches(filter[1:]) {
					enabled = false
				}
			default:
				if matches(filter) {
					enabled = true
				}
			}
		}
		return enabled
	}
	var included, excluded, forceIncluded, hasIncludes bool
	for _, filter := range filters {
		switch {
		case strings.HasPrefix(filter, "-"):
			// Exact exclusions override every other filter in normal autoload.
			if exact(filter[1:]) {
				return false
			}
		case strings.HasPrefix(filter, "+"):
			forceIncluded = forceIncluded || exact(filter[1:])
		case strings.HasPrefix(filter, "!"):
			excluded = excluded || matches(filter[1:])
		default:
			hasIncludes = true
			included = included || matches(filter)
		}
	}
	if forceIncluded {
		return true
	}
	if excluded {
		return false
	}
	if hasIncludes {
		return included
	}
	return true
}
