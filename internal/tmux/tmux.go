// Package tmux is a thin wrapper around the tmux CLI. tap always talks to
// the server the current client belongs to (via $TMUX), never a hardcoded
// socket, so tests can point it at a scratch server.
package tmux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// InsideTmux reports whether the process runs inside a tmux client.
func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// Run executes a tmux command, discarding output. Errors from unset-option
// on options that were never set are indistinguishable from real failures,
// so callers that don't care pass the error through to a blank identifier.
func Run(args ...string) error {
	return run(exec.Command("tmux", args...))
}

// Output executes a tmux command and returns its stdout without the
// trailing newline.
func Output(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := run(cmd)
	return strings.TrimRight(out.String(), "\n"), err
}

func run(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		command := strings.Join(cmd.Args[:min(2, len(cmd.Args))], " ")
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("%s: %w: %s", command, err, detail)
		}
		return fmt.Errorf("%s: %w", command, err)
	}
	return nil
}

// RunAttached executes a tmux command wired to the current terminal; used
// for display-popup, which blocks until the popup closes.
func RunAttached(args ...string) error {
	cmd := exec.Command("tmux", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
