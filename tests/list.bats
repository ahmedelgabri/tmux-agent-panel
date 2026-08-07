#!/usr/bin/env bats

load test_helper

setup() {
	start_server
}

teardown() {
	stop_server
}

@test "__list shows panes from the server" {
	tmx rename-window -t main editorwin
	run "$TAP" __list
	[ "$status" -eq 0 ]
	[[ "$output" == *"editorwin"* ]]
}

@test "__list hides the _shared session" {
	tmx new-session -d -s _shared -x 80 -y 24 'sleep 300'
	tmx rename-window -t _shared sharedwin
	run "$TAP" __list
	[ "$status" -eq 0 ]
	[[ "$output" != *"sharedwin"* ]]
}

@test "__list marks agent panes via @agent_state and current command" {
	# pane_current_command must literally be an agent name. A copied bash
	# running under the name "claude" gives tmux that name; it must block on
	# a builtin (read) because an external command would either replace the
	# process (bash -c execs a lone command) or become the name tmux picks.
	cp "$(command -v bash)" "$BATS_TEST_TMPDIR/claude"
	tmx new-window -t main -n agentwin "'$BATS_TEST_TMPDIR/claude' -c 'read x'"
	sleep 0.5
	pane="$(tmx display-message -t main:agentwin -p '#{pane_id}')"
	tmx set-option -p -t "$pane" @agent_state blocked
	tmx set-option -p -t "$pane" @agent_task 'review the diff'

	run "$TAP" __list
	[ "$status" -eq 0 ]
	[[ "$output" == *"▲"* ]]
	[[ "$output" == *"review the diff"* ]]
	[[ "$output" == *"──── agents ────"* ]]

	run "$TAP" __list --agents
	[ "$status" -eq 0 ]
	[[ "$output" == *"review the diff"* ]]
	[[ "$output" != *"──── agents ────"* ]]

	# reload children get the view from FZF_PROMPT instead of a flag
	FZF_PROMPT='agents » ' run "$TAP" __list
	[ "$status" -eq 0 ]
	[[ "$output" == *"review the diff"* ]]
	[[ "$output" != *"──── agents ────"* ]]
}

@test "__list marks agent panes via @agent_name when the command misreports" {
	# The command is plain bash — no agent name to derive — so only the
	# @agent_name option can classify the pane.
	tmx new-window -t main -n optwin "bash -c 'read x'"
	sleep 0.5
	pane="$(tmx display-message -t main:optwin -p '#{pane_id}')"
	tmx set-option -p -t "$pane" @agent_name claude
	tmx set-option -p -t "$pane" @agent_state blocked
	tmx set-option -p -t "$pane" @agent_task 'ship the flag'

	run "$TAP" __list --agents
	[ "$status" -eq 0 ]
	[[ "$output" == *"ship the flag"* ]]
	[[ "$output" == *"▲"* ]]
}

@test "doctor flags panes with orphaned agent state" {
	tmx new-window -t main -n orphanwin "bash -c 'read x'"
	sleep 0.5
	pane="$(tmx display-message -t main:orphanwin -p '#{pane_id}')"
	tmx set-option -p -t "$pane" @agent_state running

	run "$TAP" doctor
	[ "$status" -ne 0 ]
	[[ "$output" == *"$pane"* ]]
	[[ "$output" == *"no agent identity"* ]]
}

@test "__toggle flips between views" {
	FZF_PROMPT='» ' run "$TAP" __toggle
	[ "$status" -eq 0 ]
	[[ "$output" == "change-prompt(agents » )+reload("*"__list --agents)" ]]
	FZF_PROMPT='agents » ' run "$TAP" __toggle
	[[ "$output" == "change-prompt(» )+reload("*"__list)" ]]
}
