# Default: list available recipes
default:
    @just --list

# Build the binary
build:
    go build -o tap ./cmd/tap/

# Run Go unit tests
test *args:
    go test {{ args }} ./...

# Run Go unit tests with race detector
test-race *args:
    go test -race {{ args }} ./...

# Compare refresh command overhead on a private tmux server
bench-refresh *args:
    go test ./internal/picker -run '^$' -bench '^BenchmarkRefresh$' -benchmem {{ args }}

# Run the fzf library's upstream tests
test-fzf *args:
    go test {{ args }} github.com/junegunn/fzf/src/...

# Run E2E tests (bats)
test-e2e *args: build
    bats {{ args }} tests/

# Run all tests (unit + E2E)
test-all: test test-fzf test-e2e

# Run go vet
vet:
    go vet ./...

# Run staticcheck
staticcheck:
    staticcheck ./...

# Run govulncheck
govulncheck:
    govulncheck ./...

# Format all files
fmt:
    nix fmt

# Check formatting without modifying files
fmt-check:
    nix fmt -- --fail-on-change

# Run all checks (lint + tests + race + E2E + format + versions)
check: vet staticcheck test-race test-fzf test-e2e fmt-check check-versions

# Build with Nix
nix-build:
    nix build

# Run nix flake check
nix-check:
    nix flake check

# Remove build artifacts
clean:
    rm -f tap

# flake.nix is the version source of truth; mirror it into the JSON manifests
sync-versions:
    #!/usr/bin/env bash
    set -euo pipefail
    version=$(sed -nE 's/.*version = "([^"]+)".*/\1/p' flake.nix | head -1)
    for f in package.json plugins/tap-claude/.claude-plugin/plugin.json plugins/tap-codex/.claude-plugin/plugin.json; do
        jq --arg v "$version" '.version = $v' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
    done
    echo "synced version $version"

# Fail when any JSON manifest disagrees with the flake.nix version
check-versions:
    #!/usr/bin/env bash
    set -euo pipefail
    version=$(sed -nE 's/.*version = "([^"]+)".*/\1/p' flake.nix | head -1)
    status=0
    for f in package.json plugins/tap-claude/.claude-plugin/plugin.json plugins/tap-codex/.claude-plugin/plugin.json; do
        got=$(jq -r '.version' "$f")
        if [ "$got" != "$version" ]; then
            echo "$f: $got != $version (run 'just sync-versions')" >&2
            status=1
        fi
    done
    exit $status
