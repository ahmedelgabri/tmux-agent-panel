// Package picker runs the agent-aware pane picker on embedded fzf.
//
// fzf's own --tmux/--popup mode re-executes argv[0] inside the popup and
// proxies stdio over FIFOs, which cannot work with an embedded fzf (the
// re-exec would hit tap's main and Go channels don't cross processes). So
// tap wraps itself instead: outside a popup it re-executes `tap pick
// --in-popup` via `tmux display-popup -E` and the inner invocation runs fzf
// plain, owning the whole popup.
//
// An in-process poller animates busy glyphs through fzf's display fields.
// Only pane changes reload the list, after a pause in typing. fzf exports
// FZF_PROMPT to reload children so `tap __list` preserves the active view.
// --track pins the cursor across reloads.
package picker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	fzf "github.com/junegunn/fzf/src"

	"github.com/ahmedelgabri/tmux-agent-panel/internal/ansi"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/panes"
	"github.com/ahmedelgabri/tmux-agent-panel/internal/tmux"
)

// Prompts double as the view state: transforms and reload children read
// $FZF_PROMPT to tell the all-panes and agents-only views apart.
const (
	PromptAll    = "» "
	PromptAgents = "agents » "
)

var keymapFooter = " " + strings.Join([]string{
	keymap("enter", "switch pane"),
	keymap("ctrl-a", "toggle all/agents"),
	keymap("ctrl-p", "toggle preview"),
	keymap("ctrl-x", "kill pane"),
	keymap("ctrl-w", "kill window (confirm)"),
	keymap("ctrl-q", "kill session (confirm)"),
}, " · ") + ansi.Reset + " "

func keymap(key, action string) string {
	return ansi.Cyan + key + ansi.Gray + " " + action
}

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
	snapshot, err := panes.ListViews(home)
	if err != nil {
		return err
	}
	cache := filepath.Join(dir, "rows.json")
	if err := snapshot.Write(cache); err != nil {
		return err
	}
	if err := os.Setenv(panes.SnapshotEnv, cache); err != nil {
		return err
	}
	defer os.Unsetenv(panes.SnapshotEnv)
	initial, prompt := snapshot.All, PromptAll
	if snapshot.HasAgents {
		initial, prompt = snapshot.Agents, PromptAgents
	}

	opts, err := fzf.ParseOptions(false, buildArgs(self, sock, prompt))
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimRight(initial, "\n"), "\n")
	opts.Input = make(chan string, len(lines))
	for _, line := range lines {
		opts.Input <- line
	}
	close(opts.Input)

	// fzf finishes printing selections before Run returns.
	var selected []string
	opts.Printer = func(s string) { selected = append(selected, s) }

	stop := make(chan struct{})
	go refreshLoop(sock, self, cache, snapshot, func() (panes.Snapshot, error) { return panes.ListViews(home) }, stop)

	code, err := fzf.Run(opts)

	// fzf registers signal.Notify for SIGINT/SIGTERM but never calls
	// signal.Stop, so after fzf exits those signals are silently swallowed.
	// Restore default handling so Ctrl-C works again.
	signal.Reset(os.Interrupt, syscall.SIGTERM)

	close(stop)

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
		"--with-nth", panes.PickerFields(0),
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
		"--header", " ",
		"--header-border", "line",
		"--footer", " ",
		"--footer-border", "bottom",
		"--footer-label", keymapFooter,
		"--footer-label-pos", "-2",
		"--color", "bg+:-1,border:0,label:4,header-border:0,footer:8,footer-border:0,footer-label:8",
		"--bind", "ctrl-p:toggle-preview",
		"--bind", "ctrl-a:transform:" + shellQuote(self) + " __toggle",
		"--bind", "ctrl-x:execute-silent([ -n {1} ] && tmux kill-pane -t {1})+" + reload,
		"--bind", "ctrl-w:execute(" + confirmKill("window") + ")+" + reload,
		"--bind", "ctrl-q:execute(" + confirmKill("session") + ")+" + reload,
		"--preview", "[ -n {1} ] && tmux capture-pane -ep -t {1} || true",
		"--preview-window", "down,60%,border-top",
		"--preview-label-pos", "3",
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

// Pane-ID-tracked reloads block fzf input. Spinner frames instead change the
// display projection, which leaves input live. Defer actual list changes until
// query edits pause, polling all panes regardless of the active view.
func refreshLoop(sock, self, cache string, lastRows panes.Snapshot, list func() (panes.Snapshot, error), stop <-chan struct{}) {
	client := &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sock)
			},
		},
	}
	defer client.CloseIdleConnections()
	var lastQuery string
	var quietAfter time.Time
	frame := 0
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			resp, err := client.Get("http://localhost/?limit=0")
			if err != nil {
				continue
			}
			var status struct {
				Query   string `json:"query"`
				Reading bool   `json:"reading"`
			}
			err = json.NewDecoder(resp.Body).Decode(&status)
			resp.Body.Close()
			if err != nil || resp.StatusCode != http.StatusOK {
				continue
			}
			now := time.Now()
			if status.Query != lastQuery {
				lastQuery = status.Query
				quietAfter = now.Add(400 * time.Millisecond)
			}
			if status.Reading {
				continue
			}
			rows := lastRows
			if now.Before(quietAfter) {
				if !rows.Animated {
					continue
				}
			} else {
				rows, err = list()
				if err != nil {
					continue
				}
			}
			action := "refresh-preview"
			// Combining frame transforms with a reload can make fzf restore
			// pane tracking against the old rows instead of the new snapshot.
			if rows != lastRows {
				if err := rows.Write(cache); err != nil {
					continue
				}
				action = reloadAction(self)
			} else if rows.Animated {
				frame++
				action = "change-with-nth(" + panes.PickerFields(frame) + ")+" + action
			}
			resp, err = client.Post("http://localhost/", "text/plain", strings.NewReader(action))
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					lastRows = rows
				}
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
