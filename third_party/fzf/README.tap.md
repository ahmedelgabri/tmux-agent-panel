# Local fzf replacement

This is the Go library from `github.com/junegunn/fzf` v0.74.2, copied from the Go module cache with its `go.mod`, `go.sum`, licenses, Go source, assembly, and upstream unit tests. CLI-only assets and documentation are omitted. The root `go.mod` replaces the upstream module with this directory so ordinary Go builds, tests, and Nix builds all use the same fix.

## Patch

[Upstream issue #4917](https://github.com/junegunn/fzf/issues/4917): when `--listen` enables cancellable terminal reads, `LightRenderer.getch` aborts after 100 interrupted `select` calls. Active agents can trigger enough interruptions to close the picker without user input.

The only upstream source changes are:

- `src/tui/light_unix.go`: retry the terminal wait without a fixed iteration limit. `EINTR` continues the wait; cancellation and actual errors retain their existing return paths.
- `src/tui/light.go`: remove the unused `maxSelectTries` constant.

`just test-fzf` runs the copied upstream tests. `just check` includes them and a private-tmux E2E test that sends repeated `SIGURG` signals to the picker and checks that it still responds and can be cancelled. Process-directed signal delivery varies among Go threads, so the E2E test is a stress test rather than a deterministic count of interrupted waits.

Upstream files are excluded from treefmt to keep the patch auditable.

## Removal

Once an upstream release includes the fix, update the fzf requirement, remove the `replace` directive and this directory, and run `go mod tidy`. Remove the `test-fzf` recipe and its dependencies, remove the treefmt exclusion, and update the Nix vendor hash. Keep the signal stress test.
