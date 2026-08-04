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

	for _, a := range All() {
		path, err := a.ConfigPath()
		if err != nil {
			checks = append(checks, Check{a.Name, false, err.Error()})
			continue
		}
		var b strings.Builder
		detected := a.Detected()
		installed := a.Installed()
		ok := !detected || installed
		if detected {
			b.WriteString("binary found")
		} else {
			b.WriteString("binary not found")
		}
		if installed {
			fmt.Fprintf(&b, ", hooks installed (%s)", path)
		} else {
			fmt.Fprintf(&b, ", hooks not installed (%s)", path)
		}
		checks = append(checks, Check{a.Name, ok, b.String()})
	}
	return checks
}
