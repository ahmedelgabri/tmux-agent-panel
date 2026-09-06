package tmux

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tmux"), []byte("#!/bin/sh\nprintf 'partial\\n\\n'\nprintf 'cannot find pane: %%999\\n' >&2\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	for _, output := range []bool{false, true} {
		var err error
		if output {
			var got string
			got, err = Output("display-message", "-t", "%999")
			if got != "partial" {
				t.Errorf("Output = %q", got)
			}
		} else {
			err = Run("display-message", "-t", "%999")
		}
		if err == nil || !strings.Contains(err.Error(), "tmux display-message") || !strings.Contains(err.Error(), "cannot find pane: %999") {
			t.Fatalf("missing error context: %v", err)
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
			t.Fatalf("exit error not preserved: %v", err)
		}
	}
}

func TestMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := Run("list-panes"); err == nil || !strings.Contains(err.Error(), "tmux list-panes") {
		t.Fatalf("missing error context: %v", err)
	}
}
