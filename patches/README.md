# fzf terminal-wait patch

`fzf-select-eintr.patch` fixes [junegunn/fzf#4917](https://github.com/junegunn/fzf/issues/4917) in the v0.74.2 dependency pinned by `go.mod`. With `--listen`, fzf aborts after 100 interrupted `select` calls. The patch removes that limit and its unused constant. `EINTR` retries the wait; cancellation and actual errors keep their existing handling.

## Builds

- Nix applies the patch to its generated vendor directory using `buildGoModule.modPostBuild`. The vendor hash covers the patched dependency.
- `just` and release builds use `scripts/with-fzf-patch`. It downloads the pinned module through Go, patches a temporary copy, and runs the requested command with a temporary module replacement in `GOFLAGS`. It cleans up after the command finishes and never modifies the shared Go module cache or the repository's module files.
- Direct commands can use the same wrapper, for example `./scripts/with-fzf-patch go test ./internal/picker`. Plain `go build` and `go test` use unpatched upstream fzf.

`just test-fzf` runs upstream library tests against the patched dependency. `just check` includes those tests, wrapper cleanup checks, and a private-tmux stress test that sends repeated `SIGURG` signals to the picker. Signal delivery varies among Go threads, so the stress test is not a deterministic count of interrupted waits.

## Removal

Once a tested upstream release includes the fix, update the fzf requirement, remove the patch and wrapper, restore direct commands in the recipes and release workflow, and remove `modPostBuild`. Refresh the Nix vendor hash and update the source-build documentation. Keep the signal stress test.
