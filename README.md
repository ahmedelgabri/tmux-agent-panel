# tap — Tmux Agent Panel

An agent-aware tmux pane picker. `tap` lists every pane across all your tmux sessions in an fzf popup and shows, live, what your coding agents (Claude Code, Codex, pi) are doing in each of them: blocked on a permission request, waiting for your input, running, or idle — and what they're working on. Blocked agents sort first, so the pane that needs you is always at the top.

<img width="1720" height="1055" alt="Screenshot 2026-08-05 at 10 32 24" src="https://github.com/user-attachments/assets/42761dca-f216-45ae-a385-839cba365664" />

fzf is embedded as a Go library, so the only runtime dependency is tmux itself.

## Install

```sh
# Homebrew
brew install ahmedelgabri/tap/tmux-agent-panel

# mise (prebuilt binary from GitHub releases)
mise use github:ahmedelgabri/tmux-agent-panel

# Nix
nix run github:ahmedelgabri/tmux-agent-panel

# Or build from source
go build -o tap ./cmd/tap/
```

Then wire the agent hooks — this is idempotent, backs up every file it touches, and only modifies agents whose binary is on `PATH` (force a subset with `--claude`, `--codex`, `--pi`):

```sh
tap install
tap doctor # verify the wiring
```

`tap uninstall` removes exactly what `install` added and nothing else. Files managed by Nix/Home Manager (store symlinks) are refused with a pointer to declarative wiring instead.

## Usage

Open the picker from any shell inside tmux:

```sh
tap pick
```

Inside the picker: `Enter` switches to the pane, `ctrl-a` toggles an agents-only view, `?` toggles the preview, and `ctrl-x`/`ctrl-w`/`ctrl-q` kill the highlighted pane/window/session (no confirmation). The list live-refreshes 5×/second while open, so states, tasks, and the spinner animate in place.

Bind it wherever you like, e.g. a zsh widget on `C-Space`:

```sh
if [[ -n ${TMUX-} ]]; then
  tap-widget() { tap pick; zle reset-prompt }
  zle -N tap-widget
  bindkey '^@' tap-widget
fi
```

or a tmux key:

```tmux
bind-key Space run-shell 'tap pick'
```

### Reading the list

- Rows lead with an icon instead of a `session:window.pane` column: agent panes show their agent — yellow `✳` Claude Code, cyan `⌬` Codex, magenta `π` pi — and plain panes show the pane type, `❐` session or `⧉` persistent popup (`popup_*` sessions). The full address appears as the preview border label for the focused row. A blue `●` marks the pane the picker was opened from.
- State glyphs: green animated spinner running, red `▲` blocked, yellow `?` waiting, dim `◌` idle.
- The `_shared` session is hidden (its windows are linked into named sessions and would duplicate rows).

## Installing through each agent's own package manager

Instead of `tap install` editing config files, the repo doubles as a plugin/package for each agent and wires the same hooks through the agent's native channel (the `tap` binary itself still needs to be on `PATH`):

```sh
# Claude Code
/plugin marketplace add ahmedelgabri/tmux-agent-panel
/plugin install tap@tmux-agent-panel

# Codex (consumes Claude-compatible plugin marketplaces)
codex plugin marketplace add https://github.com/ahmedelgabri/tmux-agent-panel
codex plugin add tap-codex@tmux-agent-panel

# pi (the repo is the pi-tmux-agent-panel package; extensions/ is auto-discovered)
pi install https://github.com/ahmedelgabri/tmux-agent-panel

# or try it for one run without installing
pi -e git:github.com/ahmedelgabri/tmux-agent-panel
```

Pick one channel per agent — either the plugin or `tap install`, not both, or the hooks fire twice (harmless but wasteful).

## How it works

Agents report state into pane-scoped tmux user options (`@agent_state`, `@agent_task`, `@agent_name`) through hooks that invoke `tap state`. Hooks run as children of the agent process, so `$TMUX_PANE` identifies the right pane. The installed hooks pass `--agent <agent>` so the pane's agent is recorded explicitly; for panes without an `@agent_name` the picker falls back to deriving it from the pane's current command.

| Agent       | Integration                                | States                                     |
| ----------- | ------------------------------------------ | ------------------------------------------ |
| Claude Code | hook entries in `~/.claude/settings.json`  | running / idle / waiting / blocked         |
| Codex       | hook entries in `~/.codex/hooks.json`      | running / idle / blocked; task from prompt |
| pi          | extension dropped into pi's extensions dir | running / idle; task from prompt           |

When no options are set (hooks not yet active), the picker falls back to parsing Claude Code's pane title, which carries a spinner glyph while working and `✳` when waiting.

### State reporting

`tap state` is what the installed hooks (and the pi extension) call — it is the only writer of the pane options. You can also script it directly:

```sh
tap state running | idle | waiting | blocked # set @agent_state on $TMUX_PANE
tap state notification                       # route a Notification payload from stdin
tap state running --title-stdin              # also store the hook JSON's .prompt as @agent_task
tap state running --title 'some task'        # also store free text as @agent_task
tap state running --agent claude             # also record the agent as @agent_name
tap state clear                              # unset all options
```

Outside tmux every `state` invocation is a silent no-op, so hooks are safe to install unconditionally.

## Notes and caveats

- `tap install` round-trips JSON configs through Go's encoder: key order and indentation are normalized. A backup (`*.tap.bak`) is written next to each file before the first modification. Symlinked config files are followed — writes land in the target and the symlink stays intact.
- Installed hooks invoke `tap` from `PATH` (same commands the plugins ship), so upgrading or moving the binary never breaks them; `tap doctor` checks that `tap` is actually on `PATH`.
- The Claude hook set needs Claude Code ≥ 2.1.78 (when the newest wired event, `StopFailure`, shipped): before 2.1.101 an unknown hook event made Claude ignore the entire settings.json, so `tap install` refuses versions that predate any wired event rather than risk the user's config.
- Claude's `blocked` fires immediately via the `PermissionRequest` hook (the `permission_prompt` notification only fires after a few seconds of user inactivity, and still routes to `blocked` as reinforcement). Approving a request runs the tool without re-firing `PreToolUse`, so `PostToolUse`/`PostToolUseFailure` map to `running` to clear `blocked` once the tool reports back; a denial clears on the agent's next tool call or `Stop`.
- Claude's Notification routing keys on the payload's `notification_type` (permission types become `blocked`, input-needed types `waiting`, `idle_prompt` `idle`, completion types leave the state untouched); payloads without a recognized type fall back to matching English message text, degrading to `waiting`, never to a wrong `blocked`.
- Codex fires `PermissionRequest` for auto-reviewed requests too (openai/codex#28833), so it can flash a false `blocked`; it self-corrects on that call's `PostToolUse`. Codex loads hooks at session start only.
- fzf is compiled in ([`github.com/junegunn/fzf/src`](https://github.com/junegunn/fzf)) — its version is pinned at build time, so no installed fzf is needed and no version skew is possible. fzf's Go library API is not covered by stability guarantees; upgrades are deliberate, tested events.
- fzf's own `--tmux`/popup mode cannot work embedded (it re-executes argv[0] and proxies stdio over FIFOs), so `tap pick` wraps itself in `tmux display-popup` instead.

## Development

```sh
nix develop # or direnv allow
just build
just check # vet + staticcheck + unit tests (race) + bats E2E + formatting + versions
```

`flake.nix` is the version source of truth; after bumping it, run `just sync-versions` to mirror it into `package.json` and the plugin manifests. E2E tests run against a scratch tmux server on a private socket; they never touch your real tmux server or agent configs.

## License

MIT
