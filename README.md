# tap — Tmux Agent Panel

An agent-aware tmux pane picker. `tap` lists every pane across all your tmux sessions in an fzf popup and shows, live, what your coding agents (Claude Code, Codex, pi) are doing in each of them: blocked on a permission request, waiting for your input, running, or idle — and what they're working on. Blocked agents sort first, so the pane that needs you is always at the top.

<img width="1720" height="1055" alt="Screenshot 2026-08-05 at 10 32 24" src="https://github.com/user-attachments/assets/42761dca-f216-45ae-a385-839cba365664" />

fzf is embedded as a Go library, so the only runtime dependency is tmux itself.

## How it works

Agents report state into pane-scoped tmux user options (`@agent_state`, `@agent_task`) through hooks that `tap install` wires into each agent's own configuration. Hooks run as children of the agent process, so `$TMUX_PANE` identifies the right pane. Which agent a pane runs is never stored — the picker derives it from the pane's current command.

| Agent       | Integration                                | States                                     |
| ----------- | ------------------------------------------ | ------------------------------------------ |
| Claude Code | hook entries in `~/.claude/settings.json`  | running / idle / waiting / blocked         |
| Codex       | hook entries in `~/.codex/hooks.json`      | running / idle / blocked; task from prompt |
| pi          | extension dropped into pi's extensions dir | running / idle; task from prompt           |

When no options are set (hooks not yet active), the picker falls back to parsing Claude Code's pane title, which carries a spinner glyph while working and `✳` when waiting.

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

### Or install through each agent's own package manager

The repo doubles as a plugin/package for each agent, wiring the same hooks through the agent's native channel instead of `tap install` editing config files (the `tap` binary itself still needs to be on `PATH`):

```sh
# Claude Code
/plugin marketplace add ahmedelgabri/tmux-agent-panel
/plugin install tap@tmux-agent-panel

# Codex (consumes Claude-compatible plugin marketplaces)
codex plugin marketplace add https://github.com/ahmedelgabri/tmux-agent-panel
codex plugin add tap-codex@tmux-agent-panel

# pi (repo is a pi package; extensions/ is auto-discovered)
pi install https://github.com/ahmedelgabri/tmux-agent-panel

# or try it for one run without installing
pi -e git:github.com/ahmedelgabri/tmux-agent-panel
```

Pick one channel per agent — either the plugin or `tap install`, not both, or the hooks fire twice (harmless but wasteful).

Bind the picker wherever you like, e.g. a zsh widget on `C-Space`:

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

## The picker

```
tap pick
```

- Type icons instead of a `session:window.pane` column: `❐` session pane, `⧉` persistent-popup pane (`popup_*` sessions); the full address appears as the preview border label for the focused row. A blue `●` marks the pane the picker was opened from.
- Agent icons: yellow `✳` Claude Code, cyan `⌬` Codex, magenta `π` pi.
- State glyphs: green animated spinner running, red `▲` blocked, yellow `?` waiting, dim `◌` idle.
- The list live-refreshes 5×/second while open (embedded fzf's listen socket + an in-process goroutine), so states, tasks, and the spinner animate in place. `--track` pins your cursor across refreshes.
- Keys: `Enter` switch to pane, `ctrl-a` toggle agents-only view, `?` toggle preview, `ctrl-x`/`ctrl-w`/`ctrl-q` kill pane/window/session (no confirmation).
- The `_shared` session is hidden (its windows are linked into named sessions and would duplicate rows).

## State reporting

`tap state` is what the installed hooks call; you can also script it directly:

```sh
tap state running | idle | waiting | blocked # set @agent_state on $TMUX_PANE
tap state notification                       # route a Notification payload from stdin
tap state running --title-stdin              # also store the hook JSON's .prompt as @agent_task
tap state clear                              # unset both options
```

Outside tmux every `state` invocation is a silent no-op, so hooks are safe to install unconditionally.

## Notes and caveats

- `tap install` round-trips JSON configs through Go's encoder: key order and indentation are normalized. A backup (`*.tap.bak`) is written next to each file before the first modification. Symlinked config files are followed — writes land in the target and the symlink stays intact.
- Installed hooks invoke `tap` from `PATH` (same commands the plugins ship), so upgrading or moving the binary never breaks them; `tap doctor` checks that `tap` is actually on `PATH`.
- Claude's Notification routing matches English message text ("permission"); if the wording changes it degrades to `waiting`, never to a wrong `blocked`.
- Codex fires `PermissionRequest` for auto-reviewed requests too (openai/codex#28833), so it can flash a false `blocked`; it self-corrects on the next `PreToolUse`. Codex loads hooks at session start only.
- fzf is compiled in ([`github.com/junegunn/fzf/src`](https://github.com/junegunn/fzf)) — its version is pinned at build time, so no installed fzf is needed and no version skew is possible. fzf's Go library API is not covered by stability guarantees; upgrades are deliberate, tested events.
- fzf's own `--tmux`/popup mode cannot work embedded (it re-executes argv[0] and proxies stdio over FIFOs), so `tap pick` wraps itself in `tmux display-popup` instead.

## Development

```sh
nix develop # or direnv allow
just build
just check # vet + staticcheck + unit tests (race) + bats E2E + formatting
```

E2E tests run against a scratch tmux server on a private socket; they never touch your real tmux server or agent configs.

## License

MIT
