package picker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	fzf "github.com/junegunn/fzf/src"
)

func TestBuildArgs(t *testing.T) {
	args := buildArgs("/tmp/it's a tap/tap", "/tmp/test.sock", PromptAgents)
	if _, err := fzf.ParseOptions(false, args); err != nil {
		t.Fatal(err)
	}
	for i, arg := range args {
		if arg == "--id-nth" && args[i+1] == "1" {
			return
		}
	}
	t.Fatal("reload tracking needs a stable pane identity")
}

func TestShellQuote(t *testing.T) {
	for _, text := range []string{"/bin/tap", "/tmp/it's a tap/tap", "a\n$(echo unsafe);b"} {
		out, err := exec.Command("sh", "-c", "printf %s "+shellQuote(text)).Output()
		if err != nil || string(out) != text {
			t.Errorf("roundtrip %q: %q, %v", text, out, err)
		}
	}
}

func TestConfirmKill(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tmux"), []byte("#!/bin/sh\nprintf 'called:%s:%s:%s' \"$1\" \"$2\" \"$3\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, kind := range []string{"window", "session"} {
		for _, answer := range []string{"y\n", "Y\n", "n\n", "\n", ""} {
			command := strings.NewReplacer("{1}", "'%42'", "{2}", "'main:1.0'").Replace(confirmKill(kind))
			cmd := exec.Command("sh", "-c", command)
			cmd.Stdin = strings.NewReader(answer)
			out, _ := cmd.Output()
			called := strings.Contains(string(out), "called:kill-"+kind+":-t:%42")
			if want := answer == "y\n" || answer == "Y\n"; called != want {
				t.Errorf("%s answer %q: %s", kind, answer, out)
			}
		}
	}
	command := strings.NewReplacer("{1}", "''", "{2}", "''").Replace(confirmKill("session"))
	if out, _ := exec.Command("sh", "-c", command).Output(); len(out) != 0 {
		t.Fatalf("non-selectable row prompted: %s", out)
	}
}
