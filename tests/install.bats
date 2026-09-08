#!/usr/bin/env bats

load test_helper

setup() {
	# Doctor must never inspect the user's running tmux server.
	start_server
	export CLAUDE_CONFIG_DIR="$BATS_TEST_TMPDIR/claude"
	export CODEX_HOME="$BATS_TEST_TMPDIR/codex"
	export PI_CODING_AGENT_DIR="$BATS_TEST_TMPDIR/pi"
	mkdir -p "$CLAUDE_CONFIG_DIR"
	printf '{\n\t"theme": "auto"\n}\n' >"$CLAUDE_CONFIG_DIR/settings.json"
}

teardown() {
	stop_server
}

@test "install wires all forced agents" {
	run "$TAP" install --claude --codex --pi
	[ "$status" -eq 0 ]
	[ -f "$CLAUDE_CONFIG_DIR/settings.json" ]
	[ -f "$CODEX_HOME/hooks.json" ]
	[ -f "$PI_CODING_AGENT_DIR/extensions/tap-agent-state.ts" ]
	grep -q 'tap state' "$CLAUDE_CONFIG_DIR/settings.json"
	grep -q 'tap state' "$CODEX_HOME/hooks.json"
	grep -q '"theme": "auto"' "$CLAUDE_CONFIG_DIR/settings.json"
}

@test "install is idempotent" {
	"$TAP" install --claude
	first="$(cat "$CLAUDE_CONFIG_DIR/settings.json")"
	"$TAP" install --claude
	second="$(cat "$CLAUDE_CONFIG_DIR/settings.json")"
	[ "$first" = "$second" ]
}

@test "uninstall removes exactly what install added" {
	"$TAP" install --claude --codex --pi
	run "$TAP" uninstall --claude --codex --pi
	[ "$status" -eq 0 ]
	run grep -q 'tap state' "$CLAUDE_CONFIG_DIR/settings.json"
	[ "$status" -eq 1 ]
	grep -q '"theme": "auto"' "$CLAUDE_CONFIG_DIR/settings.json"
	[ ! -f "$PI_CODING_AGENT_DIR/extensions/tap-agent-state.ts" ]
}

@test "install and uninstall back up the file before every modification" {
	local pi_path="$PI_CODING_AGENT_DIR/extensions/tap-agent-state.ts"
	mkdir -p "${pi_path%/*}"
	printf '%s\n' '// hand-written extension' >"$pi_path"
	cp "$CLAUDE_CONFIG_DIR/settings.json" "$BATS_TEST_TMPDIR/original.json"
	cp "$pi_path" "$BATS_TEST_TMPDIR/original.ts"
	"$TAP" install --claude --pi
	cp "$CLAUDE_CONFIG_DIR/settings.json" "$BATS_TEST_TMPDIR/installed.json"
	cp "$pi_path" "$BATS_TEST_TMPDIR/installed.ts"
	"$TAP" uninstall --claude --pi

	backups=("$CLAUDE_CONFIG_DIR"/settings.json.*.tap.bak)
	[ "${#backups[@]}" -eq 2 ]
	# Same-second counter suffixes do not sort by creation order.
	cmp -s "$BATS_TEST_TMPDIR/original.json" "${backups[0]}" || cmp "$BATS_TEST_TMPDIR/original.json" "${backups[1]}"
	cmp -s "$BATS_TEST_TMPDIR/installed.json" "${backups[0]}" || cmp "$BATS_TEST_TMPDIR/installed.json" "${backups[1]}"

	backups=("$PI_CODING_AGENT_DIR"/extensions/tap-agent-state.ts.*.tap.bak)
	[ "${#backups[@]}" -eq 2 ]
	cmp -s "$BATS_TEST_TMPDIR/original.ts" "${backups[0]}" || cmp "$BATS_TEST_TMPDIR/original.ts" "${backups[1]}"
	cmp -s "$BATS_TEST_TMPDIR/installed.ts" "${backups[0]}" || cmp "$BATS_TEST_TMPDIR/installed.ts" "${backups[1]}"
}

@test "install preserves user formatting byte-for-byte" {
	printf '{\n  "theme": "auto",\n  "hooks": {\n    "SessionStart": [\n      {"hooks": [{"type": "command", "command": "echo hi", "async": true}]}\n    ]\n  }\n}\n' >"$CLAUDE_CONFIG_DIR/settings.json"
	cp "$CLAUDE_CONFIG_DIR/settings.json" "$BATS_TEST_TMPDIR/original.json"

	"$TAP" install --claude
	grep -q '"theme": "auto",' "$CLAUDE_CONFIG_DIR/settings.json"
	grep -q 'tap state' "$CLAUDE_CONFIG_DIR/settings.json"

	"$TAP" uninstall --claude
	diff "$BATS_TEST_TMPDIR/original.json" "$CLAUDE_CONFIG_DIR/settings.json"
}

@test "doctor reports wiring status" {
	"$TAP" install --claude --codex --pi
	run "$TAP" doctor
	[[ "$output" == *"tmux"* ]]
	[[ "$output" == *"hooks installed"* ]]
	[[ "$output" != *"outdated"* ]]
}

@test "install refuses a claude that breaks on unknown hook events" {
	mkdir -p "$BATS_TEST_TMPDIR/bin"
	printf '#!/bin/sh\necho "2.1.77 (Claude Code)"\n' >"$BATS_TEST_TMPDIR/bin/claude"
	chmod +x "$BATS_TEST_TMPDIR/bin/claude"
	run env PATH="$BATS_TEST_TMPDIR/bin:$PATH" "$TAP" install --claude
	[ "$status" -ne 0 ]
	[[ "$output" == *"2.1.78"* ]]
	run grep -q 'tap state' "$CLAUDE_CONFIG_DIR/settings.json"
	[ "$status" -eq 1 ]
}

@test "install migrates hooks out of events no longer in the set" {
	"$TAP" install --claude
	# Simulate hooks from a tap version that hooked a since-dropped event.
	sed -i.orig 's/"PreToolUse"/"TapObsoleteEvent"/' "$CLAUDE_CONFIG_DIR/settings.json"
	"$TAP" install --claude
	run grep -q 'TapObsoleteEvent' "$CLAUDE_CONFIG_DIR/settings.json"
	[ "$status" -eq 1 ]
	grep -q '"PreToolUse"' "$CLAUDE_CONFIG_DIR/settings.json"
	run "$TAP" doctor
	[[ "$output" != *"outdated"* ]]
}

@test "doctor flags outdated hooks" {
	"$TAP" install --claude --codex --pi
	# Simulate hooks from a tap version that predates --agent.
	sed -i.orig 's/ --agent claude//g' "$CLAUDE_CONFIG_DIR/settings.json"
	run "$TAP" doctor
	[[ "$output" == *"outdated"* ]]
	[[ "$output" == *"tap install"* ]]
}
