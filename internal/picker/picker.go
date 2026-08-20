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
			fmt.Sprintf("'%s' pick --in-popup", self))
	}

	sock := filepath.Join(os.TempDir(), fmt.Sprintf("tap-%d.sock", os.Getpid()))
	defer os.Remove(sock)

	opts, err := fzf.ParseOptions(false, buildArgs(self, sock))
	if err != nil {
		return err
	}

	// The pane the picker was opened from cannot change while the popup is
	// open; resolve it once and let every reload child inherit it.
	if current, err := tmux.Output("display-message", "-p", "#{pane_id}"); err == nil {
		os.Setenv(panes.CurrentPaneEnv, current)
	}

	home, _ := os.UserHomeDir()
	initial, err := panes.List(false, home)
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

	// The orphan warning row has an empty pane_id field, making Enter on it a no-op.
	paneID, _, _ := strings.Cut(selected[0], "\t")
	if strings.HasPrefix(paneID, "%") {
		return tmux.Run("switch-client", "-Z", "-t", paneID)
	}
	return nil
}

func buildArgs(self, sock string) []string {
	reload := reloadAction(self)
	return []string{
		"--ansi",
		"--reverse",
		"--padding", "1,2",
		"--delimiter", "\t",
		"--with-nth", "3..",
		"--track",
		"--gutter", " ",
		"--gutter-raw", " ",
		"--listen", sock,
		"--prompt", PromptAll,
		"--pointer", "▶",
		"--info", "inline-right",
		"--separator", "",
		// The header label doesn't render without a header window, hence
		// the blank header.
		"--header", " ",
		"--header-border", "line",
		"--header-label", "ctrl-a agents · ? preview · kill: ctrl-x pane · ctrl-w window · ctrl-q session",
		"--color", "bg+:-1,border:0,label:4,header-border:0,header-label:8",
		"--bind", "?:toggle-preview",
		"--bind", fmt.Sprintf("ctrl-a:transform:'%s' __toggle", self),
		"--bind", "ctrl-x:execute-silent([ -n {1} ] && tmux kill-pane -t {1})+" + reload,
		"--bind", "ctrl-w:execute-silent([ -n {1} ] && tmux kill-window -t {1})+" + reload,
		"--bind", "ctrl-q:execute-silent([ -n {1} ] && tmux kill-session -t {1})+" + reload,
		"--preview", "[ -n {1} ] && tmux capture-pane -ep -t {1} || true",
		"--preview-window", "down,60%,border-top",
		"--bind", `focus:transform-preview-label:printf " %s " {2}`,
	}
}

// reloadAction re-runs `tap __list`, which reads $FZF_PROMPT to preserve
// whichever view (all/agents) is active. Both the kill bindings and the
// refresh loop use it.
func reloadAction(self string) string {
	return fmt.Sprintf("reload('%s' __list)", self)
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
		return fmt.Sprintf("change-prompt(%s)+reload('%s' __list --agents)", PromptAgents, self)
	}
	return fmt.Sprintf("change-prompt(%s)+reload('%s' __list)", PromptAll, self)
}
