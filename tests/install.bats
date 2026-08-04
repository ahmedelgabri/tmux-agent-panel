#!/usr/bin/env bats

load test_helper

setup() {
	export CLAUDE_CONFIG_DIR="$BATS_TEST_TMPDIR/claude"
	export CODEX_HOME="$BATS_TEST_TMPDIR/codex"
	export PI_CODING_AGENT_DIR="$BATS_TEST_TMPDIR/pi"
	mkdir -p "$CLAUDE_CONFIG_DIR"
	printf '{\n\t"theme": "auto"\n}\n' >"$CLAUDE_CONFIG_DIR/settings.json"
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
	! grep -q 'tap state' "$CLAUDE_CONFIG_DIR/settings.json"
	grep -q '"theme": "auto"' "$CLAUDE_CONFIG_DIR/settings.json"
	[ ! -f "$PI_CODING_AGENT_DIR/extensions/tap-agent-state.ts" ]
}

@test "install writes a backup before modifying" {
	"$TAP" install --claude
	[ -f "$CLAUDE_CONFIG_DIR/settings.json.tap.bak" ]
	grep -q '"theme": "auto"' "$CLAUDE_CONFIG_DIR/settings.json.tap.bak"
}

@test "doctor reports wiring status" {
	"$TAP" install --claude --codex --pi
	run "$TAP" doctor
	[[ "$output" == *"tmux"* ]]
	[[ "$output" == *"hooks installed"* ]]
}
