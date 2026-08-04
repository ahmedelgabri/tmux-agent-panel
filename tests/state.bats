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
	"$TAP" state blocked
	run "$TAP" state clear
	[ "$status" -eq 0 ]
	# -q: unset user options otherwise make show-options error
	run tmx show-options -pqv -t "$TMUX_PANE" @agent_state
	[ -z "$output" ]
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

@test "title-stdin stores the prompt as @agent_task" {
	run bash -c "echo '{\"prompt\":\"fix the flaky test\"}' | '$TAP' state running --title-stdin"
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_task
	[ "$output" = "fix the flaky test" ]
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
