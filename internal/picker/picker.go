// Package picker runs the agent-aware pane picker on embedded fzf.
//
// fzf's own --tmux/--popup mode re-executes argv[0] inside the popup and
// proxies stdio over FIFOs, which cannot work with an embedded fzf (the
// re-exec would hit tap's main and Go channels don't cross processes). So
// tap wraps itself instead: outside a popup it re-executes `tap pick
// --in-popup` via `tmux display-popup -E` and the inner invocation runs fzf
// plain, owning the whole popup.
//
// fzf listens on a Unix socket and an in-process goroutine POSTs a reload
// action every 200ms, animating the running-state spinner and keeping
// agent states live while the picker is open. fzf exports FZF_PROMPT to
// reload children, so `tap __list` derives the active view (all vs
// agents-only) itself — no transform indirection needed. --track pins the
// cursor across reloads.
package picker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	fzf "github.com/junegunn/fzf/src"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/panes"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/tmux"
)

// Prompts double as the view state: transforms and reload children read
// $FZF_PROMPT to tell the all-panes and agents-only views apart.
const (
	PromptAll    = "» "
	PromptAgents = "agents » "
)

// Run shows the picker and switches to the chosen pane.
func Run(inPopup bool) error {
	if !tmux.InsideTmux() {
		return errors.New("must be run inside tmux")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}

	if !inPopup {
		return tmux.RunAttached("display-popup", "-E", "-w", "85%", "-h", "85%",
			shellQuote(self)+" pick --in-popup")
	}

	dir, err := os.MkdirTemp("", "tap-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "fzf.sock")

	// The pane the picker was opened from cannot change while the popup is
	// open; resolve it once and let every reload child inherit it.
	if current, err := tmux.Output("display-message", "-p", "#{pane_id}"); err == nil {
		os.Setenv(panes.CurrentPaneEnv, current)
	}

	// The picker opens focused on agents; ctrl-a widens to all panes. With
	// no agent panes the focused view would be empty, so start on all.
	home, _ := os.UserHomeDir()
	initial, agentsView, err := panes.ListInitial(home)
	if err != nil {
		return err
	}
	prompt := PromptAll
	if agentsView {
		prompt = PromptAgents
	}

	opts, err := fzf.ParseOptions(false, buildArgs(self, sock, prompt))
	if err != nil {
		return err
	}

	inputChan := make(chan string)
	go func() {
		for _, line := range strings.Split(strings.TrimRight(initial, "\n"), "\n") {
			inputChan <- line
		}
		close(inputChan)
	}()

	outputChan := make(chan string, 8)
	var selected []string
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for s := range outputChan {
			selected = append(selected, s)
		}
	}()

	opts.Input = inputChan
	opts.Output = outputChan

	stop := make(chan struct{})
	go refreshLoop(sock, self, stop)

	code, err := fzf.Run(opts)

	// fzf registers signal.Notify for SIGINT/SIGTERM but never calls
	// signal.Stop, so after fzf exits those signals are silently swallowed.
	// Restore default handling so Ctrl-C works again.
	signal.Reset(os.Interrupt, syscall.SIGTERM)

	close(stop)
	// fzf does not close the Output channel, so the goroutine draining it
	// would block forever on the range loop. Close it now that fzf is done.
	close(outputChan)
	wg.Wait()

	if err != nil && code != fzf.ExitInterrupt {
		return err
	}
	if code == fzf.ExitInterrupt || code == fzf.ExitNoMatch || len(selected) == 0 {
		return nil
	}

	// Non-selectable rows (spacer, orphan warning) have an empty pane_id
	// field, making Enter on them a no-op.
	paneID, _, _ := strings.Cut(selected[0], "\t")
	if strings.HasPrefix(paneID, "%") {
		return tmux.Run("switch-client", "-Z", "-t", paneID)
	}
	return nil
}

func buildArgs(self, sock, prompt string) []string {
	reload := reloadAction(self)
	return []string{
		"--ansi",
		"--reverse",
		"--padding", "1,2",
		"--delimiter", "\t",
		"--with-nth", "3..",
		"--track",
		"--id-nth", "1",
		"--with-shell", "sh -c",
		"--gutter", " ",
		"--gutter-raw", " ",
		"--listen", sock,
		"--prompt", prompt,
		"--pointer", "▶",
		"--info", "inline-right",
		"--separator", "",
		// The header label doesn't render without a header window, hence
		// the blank header.
		"--header", " ",
		"--header-border", "line",
		"--header-label", "ctrl-a all/agents · ? preview · kill: ctrl-x pane · confirm: ctrl-w window · ctrl-q session",
		"--color", "bg+:-1,border:0,label:4,header-border:0,header-label:8",
		"--bind", "?:toggle-preview",
		"--bind", "ctrl-a:transform:" + shellQuote(self) + " __toggle",
		"--bind", "ctrl-x:execute-silent([ -n {1} ] && tmux kill-pane -t {1})+" + reload,
		"--bind", "ctrl-w:execute(" + confirmKill("window") + ")+" + reload,
		"--bind", "ctrl-q:execute(" + confirmKill("session") + ")+" + reload,
		"--preview", "[ -n {1} ] && tmux capture-pane -ep -t {1} || true",
		"--preview-window", "down,60%,border-top",
		"--bind", `focus:transform-preview-label:printf " %s " {2}`,
	}
}

// reloadAction re-runs `tap __list`, which reads $FZF_PROMPT to preserve
// whichever view (all/agents) is active. Both the kill bindings and the
// refresh loop use it.
func reloadAction(self string) string {
	return "reload(" + shellQuote(self) + " __list)"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// fzf expands placeholders before suspending for execute, so confirmation
// stays attached to the original pane even if live updates reorder the list.
func confirmKill(kind string) string {
	return fmt.Sprintf(`[ -n {1} ] && printf 'Kill %s containing %%s? [y/N] ' {2} && read -r answer && { [ "$answer" = y ] || [ "$answer" = Y ]; } && tmux kill-%s -t {1}`, kind, kind)
}

// refreshLoop drives live updates: every 200ms it POSTs a reload action to
// fzf's listen socket. Dial errors are expected both before fzf binds the
// socket and after it exits, so they are ignored.
func refreshLoop(sock, self string, stop <-chan struct{}) {
	client := &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sock)
			},
		},
	}
	action := reloadAction(self)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			resp, err := client.Post("http://localhost/", "text/plain", strings.NewReader(action))
			if err == nil {
				resp.Body.Close()
			}
		}
	}
}

// TransformToggle emits the fzf actions that flip between the all-panes and
// agents-only views. Meant to run as an fzf transform binding, which
// provides $FZF_PROMPT. The reload passes the view explicitly rather than
// relying on the prompt, which changes in the same action chain.
func TransformToggle(self string) string {
	if os.Getenv("FZF_PROMPT") == PromptAll {
		return fmt.Sprintf("change-prompt(%s)+reload(%s __list --agents)", PromptAgents, shellQuote(self))
	}
	return fmt.Sprintf("change-prompt(%s)+reload(%s __list)", PromptAll, shellQuote(self))
}
