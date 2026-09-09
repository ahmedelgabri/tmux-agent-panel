#!/usr/bin/env bats

@test "build wrapper patches a private copy without changing the module cache" {
	local root source before
	root="$BATS_TEST_DIRNAME/.."
	source="$(go -C "$root" list -m -f '{{.Dir}}' github.com/junegunn/fzf)"
	before="$(cksum "$source/src/tui/light.go" "$source/src/tui/light_unix.go")"
	run "$root/scripts/with-fzf-patch" sh -ec '
		source=$(go list -m -f "{{.Dir}}" github.com/junegunn/fzf)
		! grep -q maxSelectTries "$source/src/tui/light.go" "$source/src/tui/light_unix.go"
		printf "%s\n" "$source"
	'
	[ "$status" -eq 0 ]
	[[ "$output" == /tmp/tap-fzf.*/fzf ]]
	[ ! -e "$output" ]
	[ "$(cksum "$source/src/tui/light.go" "$source/src/tui/light_unix.go")" = "$before" ]
}

@test "build wrapper preserves failures and cleans up the temporary module" {
	run "$BATS_TEST_DIRNAME/../scripts/with-fzf-patch" sh -c '
		go list -m -f "{{.Dir}}" github.com/junegunn/fzf
		exit 7
	'
	[ "$status" -eq 7 ]
	[[ "$output" == /tmp/tap-fzf.*/fzf ]]
	[ ! -e "$output" ]
}
