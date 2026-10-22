// Package command runs the CLIs tap drives (tmux, rex, ps) and folds their
// stderr into the returned error, since that is where they explain failures.
package command

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes a command, discarding output.
func Run(name string, args ...string) error {
	return run(exec.Command(name, args...))
}

// Output executes a command and returns its stdout without the trailing
// newline.
func Output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
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
