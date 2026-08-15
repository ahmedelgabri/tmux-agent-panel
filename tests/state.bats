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
	run bash -c "echo '{\"message\":\"Claude needs your permission\"}' | '$TAP' state notification"
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_state
	[ "$output" = "blocked" ]
}

@test "notification without permission message becomes waiting" {
	run bash -c "echo '{\"message\":\"Waiting for your input\"}' | '$TAP' state notification"
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_state
	[ "$output" = "waiting" ]
}

@test "notification routes on notification_type over message text" {
	run bash -c "echo '{\"notification_type\":\"permission_prompt\",\"message\":\"localized wording\"}' | '$TAP' state notification"
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_state
	[ "$output" = "blocked" ]
}

@test "worker permission notification becomes blocked" {
	run bash -c "echo '{\"notification_type\":\"worker_permission_prompt\",\"message\":\"my-worker needs permission for Bash\"}' | '$TAP' state notification"
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_state
	[ "$output" = "blocked" ]
}

@test "idle_prompt notification becomes idle" {
	run bash -c "echo '{\"notification_type\":\"idle_prompt\",\"message\":\"Claude is waiting for your input\"}' | '$TAP' state notification"
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_state
	[ "$output" = "idle" ]
}

@test "completion notification leaves pane state untouched" {
	"$TAP" state running
	run bash -c "echo '{\"notification_type\":\"elicitation_complete\"}' | '$TAP' state notification"
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_state
	[ "$output" = "running" ]
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
