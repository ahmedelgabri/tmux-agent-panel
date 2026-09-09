package agents

import (
	"cmp"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// Codex permits non-SemVer cache names, but only safe ASCII path segments.
func validCodexPluginVersion(version string) bool {
	if version == "" || version == "." || version == ".." {
		return false
	}
	for _, c := range version {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.+", c)) {
			return false
		}
	}
	return true
}

func compareCodexPluginVersions(left, right string) int {
	a, b := codexSemver(left), codexSemver(right)
	if a == "" || b == "" {
		return strings.Compare(left, right)
	}
	if order := semver.Compare(a, b); order != 0 {
		return order
	}
	// Rust's semver::Version::cmp includes build metadata as a tiebreaker,
	// unlike SemVer precedence. Numeric identifiers sort before text.
	return compareCodexBuild(strings.TrimPrefix(semver.Build(a), "+"), strings.TrimPrefix(semver.Build(b), "+"))
}

func codexSemver(version string) string {
	v := "v" + version
	if !semver.IsValid(v) {
		return ""
	}
	// Rust requires all three components, each fitting in a u64; x/mod
	// also accepts shortened versions and arbitrarily large components.
	core, _, _ := strings.Cut(version, "+")
	core, _, _ = strings.Cut(core, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return ""
	}
	for _, part := range parts {
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return ""
		}
	}
	return v
}

func compareCodexBuild(left, right string) int {
	a, b := strings.Split(left, "."), strings.Split(right, ".")
	for i := range min(len(a), len(b)) {
		x, y := a[i], b[i]
		xNum, yNum := strings.Trim(x, "0123456789") == "", strings.Trim(y, "0123456789") == ""
		switch {
		case xNum && yNum:
			xn, yn := strings.TrimLeft(x, "0"), strings.TrimLeft(y, "0")
			if order := cmp.Compare(len(xn), len(yn)); order != 0 {
				return order
			}
			if order := strings.Compare(xn, yn); order != 0 {
				return order
			}
			if order := cmp.Compare(len(x), len(y)); order != 0 {
				return order
			}
		case xNum:
			return -1
		case yNum:
			return 1
		default:
			if order := strings.Compare(x, y); order != 0 {
				return order
			}
		}
	}
	return cmp.Compare(len(a), len(b))
}
