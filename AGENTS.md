# AGENTS.md

`tap` (Tmux Agent Panel) is a Go CLI that shows an fzf-based tmux pane picker with live coding-agent status (Claude Code, Codex, pi), records agent state into pane-scoped tmux user options, and installs the agent-side hooks that feed it.

This repository uses the Jujutsu version control system (`jj`), colocated with git.

## Layout

- `cmd/tap/` — main entry point
- `internal/cmd/` — cobra commands (`pick`, `state`, `install`, `uninstall`, `doctor`, hidden `__list`/`__reload`/`__toggle`)
- `internal/panes/` — pane listing and row rendering (the picker's model)
- `internal/picker/` — embedded fzf UI (`github.com/junegunn/fzf/src`)
- `internal/state/` — writes `@agent_state`/`@agent_task` pane options
- `internal/agents/` — hook installers for Claude Code, Codex, and pi
- `internal/tmux/` — thin tmux command wrapper
- `tests/` — bats end-to-end tests (run against a scratch tmux server)

## Conventions

- Smallest reasonable changes; match surrounding style.
- Comments describe "why", not "what"; no temporal references.
- No trailing whitespace, including on blank lines.
- Conventional Commits (`feat:`, `fix:`, `chore:`, scoped when it helps).
- Do not hard-wrap Markdown.
- `just check` must pass before pushing (vet, staticcheck, tests, e2e, formatting).

## Development

Enter the devshell with `direnv` or `nix develop`, then use `just` recipes (`just build`, `just test`, `just test-e2e`, `just check`).
