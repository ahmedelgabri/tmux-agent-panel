---
layout: default
title: tap
---

# tap — Tmux Agent Panel

An agent-aware tmux pane picker. `tap` lists every pane across all your tmux sessions in an fzf popup and shows, live, what your coding agents (Claude Code, Codex, pi) are doing in each of them: blocked on a permission request, waiting for your input, running, or idle — and what they're working on. Blocked agents sort first, so the pane that needs you is always at the top.

<img alt="The tap picker: agent panes with live state glyphs sorted first, plain panes below" src="https://github.com/user-attachments/assets/42761dca-f216-45ae-a385-839cba365664" />

fzf is embedded as a Go library, so the only runtime dependency is tmux itself.

## Features

- **Live agent status** per pane: green `⠋` running, red `▲` blocked, yellow `?` waiting, dim `◌` idle. State is polled every 200 ms; the list reloads only when rows change, after a pause in typing. Preview output refreshes independently.
- **Blocked-first ordering** so permission requests surface immediately
- **Hook installation** with `tap install` backs up existing JSON configs before every rewrite, including byte-identical reinstalls. `tap uninstall` also backs up JSON configs before removing direct tap hooks. Backups use `<file>.<YYYYMMDDTHHMMSS>.tap.bak`, with `-1`, `-2`, etc. before `.tap.bak` for same-second collisions; none are overwritten or pruned. The managed pi extension is overwritten without a backup and removed by `tap uninstall`.
- **Agent-native install channels**: Claude Code plugin, Codex plugin, and a pi package (`pi-tmux-agent-panel`)
- **Embedded fzf** — no fzf installation, no version skew
- **Agent-focused by default.** `ctrl-a` toggles all panes and `?` toggles the preview. Selection follows the pane across reordered updates. `ctrl-x` kills a pane immediately; `ctrl-w` and `ctrl-q` ask for confirmation before killing a window or session.
- **Diagnostics** with `tap doctor` check user-scoped direct hooks and native plugin/package installs. Project-scoped installs and one-session CLI overrides are not checked.

## Quick Start

```bash
# Homebrew
brew install ahmedelgabri/tap/tmux-agent-panel

# mise
mise use github:ahmedelgabri/tmux-agent-panel

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

Or wire them through each agent's own package manager instead — the repo doubles as a plugin/package for all three:

```bash
# Claude Code
/plugin marketplace add ahmedelgabri/tmux-agent-panel
/plugin install tap@tmux-agent-panel

# Codex (consumes Claude-compatible plugin marketplaces)
codex plugin marketplace add https://github.com/ahmedelgabri/tmux-agent-panel
codex plugin add tap-codex@tmux-agent-panel

# pi (the repo is the pi-tmux-agent-panel package)
pi install https://github.com/ahmedelgabri/tmux-agent-panel
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

Agents report state into pane-scoped tmux user options through hooks that invoke `tap state`, the single writer across all three agents. `@agent_state` records activity, `@agent_task` records the task, and `@agent_name` identifies the agent. Hooks inherit `$TMUX_PANE` from the agent process to target the right pane. The picker prefers the recorded identity and falls back to the pane's current command when no recognized agent name is stored.

| Agent       | Integration                                | States                                     |
| ----------- | ------------------------------------------ | ------------------------------------------ |
| Claude Code | hook entries in `~/.claude/settings.json`  | running / idle / waiting / blocked         |
| Codex       | hook entries in `~/.codex/hooks.json`      | running / idle / blocked; task from prompt |
| pi          | extension dropped into pi's extensions dir | running / idle; task from prompt           |

See the [README](https://github.com/ahmedelgabri/tmux-agent-panel#readme) for the full documentation.

## License

MIT © [Ahmed El Gabri](https://gabri.me)
