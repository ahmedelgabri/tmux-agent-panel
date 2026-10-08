#!/usr/bin/env bats

load test_helper

setup() {
	start_server
}

teardown() {
	stop_server
}

@test "state sets @agent_state on the pane" {
	run "$TAP" state running
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_state
	[ "$output" = "running" ]
}

@test "state clear unsets the options" {
	"$TAP" state blocked --agent claude
	run "$TAP" state clear
	[ "$status" -eq 0 ]
	# -q: unset user options otherwise make show-options error
	run tmx show-options -pqv -t "$TMUX_PANE" @agent_state
	[ -z "$output" ]
	run tmx show-options -pqv -t "$TMUX_PANE" @agent_name
	[ -z "$output" ]
}

@test "agent flag sets @agent_name on the pane" {
	run "$TAP" state running --agent codex
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_name
	[ "$output" = "codex" ]
}

@test "unknown agent fails" {
	run "$TAP" state running --agent bogus
	[ "$status" -ne 0 ]
	[[ "$output" == *"invalid agent"* ]]
}

@test "notification with permission message becomes blocked" {
	assert_notification_state '{"message":"Claude needs your permission"}' blocked
}

@test "notification without permission message becomes waiting" {
	assert_notification_state '{"message":"Waiting for your input"}' waiting
}

@test "notification routes on notification_type over message text" {
	assert_notification_state '{"notification_type":"permission_prompt","message":"localized wording"}' blocked
}

@test "worker permission notification becomes blocked" {
	assert_notification_state '{"notification_type":"worker_permission_prompt","message":"my-worker needs permission for Bash"}' blocked
}

@test "idle_prompt notification becomes idle" {
	assert_notification_state '{"notification_type":"idle_prompt","message":"Claude is waiting for your input"}' idle
}

@test "completion notification leaves pane state untouched" {
	"$TAP" state running
	assert_notification_state '{"notification_type":"elicitation_complete"}' running
}

@test "title-stdin stores the prompt as @agent_task" {
	run bash -c "echo '{\"prompt\":\"fix the flaky test\"}' | '$TAP' state running --title-stdin"
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_task
	[ "$output" = "fix the flaky test" ]
}

@test "title stores free text as @agent_task" {
	run "$TAP" state running --title 'refactor the picker'
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_task
	[ "$output" = "refactor the picker" ]
}

@test "invalid state fails" {
	run "$TAP" state bogus
	[ "$status" -ne 0 ]
}

@test "tmux failures include the command and diagnostic" {
	TMUX_PANE='%999999' run "$TAP" state running
	[ "$status" -ne 0 ]
	[[ "$output" == *"tmux set-option"* ]]
	[[ "$output" == *"%999999"* ]]
}

@test "outside tmux is a silent no-op" {
	run env -u TMUX -u TMUX_PANE "$TAP" state running
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "osc7501 opt-in mirrors state to the pane's tty" {
	tmx set-option -g @tap_osc7501 on
	tmx pipe-pane -o -t "$TMUX_PANE" "cat > '$TMUX_TEST_DIR/out'"
	"$TAP" state blocked --agent claude
	# Escapes doubled for tmux passthrough; the task-less report has no msg.
	expected=$'\ePtmux;\e\e]7501;state=blocked:kind=permission:id=pane-'"${TMUX_PANE#%}"$':app=claude-code\e\e\\\e\\'
	wait_for grep -qF "$expected" "$TMUX_TEST_DIR/out"
}

@test "osc7501 stays silent without the opt-in" {
	tmx pipe-pane -o -t "$TMUX_PANE" "cat > '$TMUX_TEST_DIR/out'"
	"$TAP" state blocked --agent claude
	"$TAP" state clear
	run tmx show-options -pqv -t "$TMUX_PANE" @agent_state
	[ -z "$output" ]
	! grep -q 7501 "$TMUX_TEST_DIR/out" 2>/dev/null
}
