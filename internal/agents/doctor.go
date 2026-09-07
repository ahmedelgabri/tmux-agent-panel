package agents

import (
	"fmt"
	"os/exec"
	"strings"
)

// Check is one doctor finding.
type Check struct {
	Name   string
	OK     bool
	Detail string
}

// Doctor reports the health of the whole integration: runtime dependencies
// and, per agent, whether the binary exists and hooks are wired.
func Doctor() []Check {
	var checks []Check

	if path, err := exec.LookPath("tmux"); err == nil {
		version, _ := exec.Command("tmux", "-V").Output()
		checks = append(checks, Check{"tmux", true, path + " (" + strings.TrimSpace(string(version)) + ")"})
	} else {
		checks = append(checks, Check{"tmux", false, "not found on PATH — tap cannot work without it"})
	}

	// Hooks and plugins invoke `tap` by name, so PATH is part of the contract.
	if path, err := exec.LookPath("tap"); err == nil {
		checks = append(checks, Check{"tap", true, path + " (hooks invoke it by name)"})
	} else {
		checks = append(checks, Check{"tap", false, "not on PATH — installed hooks and plugins invoke `tap` by name"})
	}

	for _, a := range All() {
		path, err := a.ConfigPath()
		if err != nil {
			checks = append(checks, Check{a.Name, false, err.Error()})
			continue
		}
		var b strings.Builder
		detected := a.Detected()
		installed := a.Installed()
		current := installed && a.Current()
		native, nativeErr := a.Native()
		ok := current || native != ""
		if detected {
			b.WriteString("binary found")
		} else {
			b.WriteString("binary not found")
		}
		switch {
		case current:
			fmt.Fprintf(&b, ", hooks installed (%s)", path)
		case installed:
			ok = false
			if native != "" && nativeErr == nil {
				fmt.Fprintf(&b, ", hooks outdated. Remove direct wiring with `tap uninstall --%s` (%s)", a.Name, path)
			} else {
				fmt.Fprintf(&b, ", hooks outdated. Run `tap install` (%s)", path)
			}
		case native == "" && nativeErr == nil:
			fmt.Fprintf(&b, ", user hooks not installed (%s)", path)
		}
		if native != "" {
			fmt.Fprintf(&b, ", %s", native)
			if installed {
				b.WriteString("; both install channels are wired, disable one to avoid duplicate hooks")
			}
		}
		if nativeErr != nil {
			ok = false
			fmt.Fprintf(&b, ", %v", nativeErr)
		}
		checks = append(checks, Check{a.Name, !detected || ok, b.String()})
	}
	return checks
}
