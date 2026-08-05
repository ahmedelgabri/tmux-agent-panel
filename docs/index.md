---
layout: default
title: tap
---

# tap — Tmux Agent Panel

An agent-aware tmux pane picker. `tap` lists every pane across all your tmux sessions in an fzf popup and shows, live, what your coding agents (Claude Code, Codex, pi) are doing in each of them: blocked on a permission request, waiting for your input, running, or idle — and what they're working on. Blocked agents sort first, so the pane that needs you is always at the top.

fzf is embedded as a Go library, so the only runtime dependency is tmux itself.

## Features

- **Live agent status** per pane: green spinner running, red `▲` blocked, yellow `?` waiting, dim `◌` idle — refreshed 5×/second while the picker is open
- **Blocked-first ordering** so permission requests surface immediately
- **One-command hook installation** with `tap install`: idempotent, backed up, reversible with `tap uninstall`
- **Embedded fzf** — no fzf installation, no version skew
- **Agents-only view** (`ctrl-a`), pane preview (`?`), kill pane/window/session from the picker
- **Diagnostics** with `tap doctor`

## Quick Start

```bash
# Homebrew
brew install ahmedelgabri/tap/tmux-agent-panel

# mise
mise use -g "ubi:ahmedelgabri/tmux-agent-panel[exe=tap]"

# Nix Flakes
nix run github:ahmedelgabri/tmux-agent-panel

# Or build from source
go build -o tap ./cmd/tap/
```

Wire the agent hooks and verify:

```bash
tap install
tap doctor
```

Open the picker from any shell inside tmux:

```bash
tap pick
```

Bind it to a key, e.g. a zsh widget on `C-Space`:

```sh
if [[ -n ${TMUX-} ]]; then
  tap-widget() { tap pick; zle reset-prompt }
  zle -N tap-widget
  bindkey '^@' tap-widget
fi
```

## How it works

Agents report state into pane-scoped tmux user options (`@agent_state`, `@agent_task`) through hooks that `tap install` wires into each agent's own configuration. Hooks run as children of the agent process, so `$TMUX_PANE` identifies the right pane. Which agent a pane runs is never stored — the picker derives it from the pane's current command.

| Agent       | Integration                                | States                                     |
| ----------- | ------------------------------------------ | ------------------------------------------ |
| Claude Code | hook entries in `~/.claude/settings.json`  | running / idle / waiting / blocked         |
| Codex       | hook entries in `~/.codex/hooks.json`      | running / idle / blocked; task from prompt |
| pi          | extension dropped into pi's extensions dir | running / idle; task from prompt           |

See the [README](https://github.com/ahmedelgabri/tmux-agent-panel#readme) for the full documentation.

## License

MIT © [Ahmed El Gabri](https://gabri.me)
