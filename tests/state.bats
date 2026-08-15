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

@test "outside tmux is a silent no-op" {
	run env -u TMUX -u TMUX_PANE "$TAP" state running
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}
