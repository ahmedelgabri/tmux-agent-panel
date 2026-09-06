#!/usr/bin/env bats

load test_helper

setup() {
	start_server
	tmx set-option -p -t "$TMUX_PANE" @agent_name codex
	tmx set-option -p -t "$TMUX_PANE" @agent_state running
	SECOND="$(tmx new-window -t main -n second -P -F '#{pane_id}' 'sleep 300')"
	tmx set-option -p -t "$SECOND" @agent_name pi
	tmx set-option -p -t "$SECOND" @agent_state idle
	PICKER="$(tmx new-window -t main -n picker -P -F '#{pane_id}' "env TMPDIR='$TMUX_TEST_DIR' '$TAP' pick --in-popup")"
	wait_for picker_ready
}

teardown() {
	stop_server
}

picker_ready() {
	local socket
	for socket in "$TMUX_TEST_DIR"/tap-*/fzf.sock; do
		if [ -S "$socket" ]; then
			FZF_SOCKET="$socket"
			picker_status | jq -e '.matchCount == 2' >/dev/null && return 0
		fi
	done
	return 1
}

picker_status() {
	curl --silent --show-error --max-time 1 --unix-socket "$FZF_SOCKET" http://localhost/
}

picker_action() {
	curl --silent --show-error --max-time 1 --unix-socket "$FZF_SOCKET" -X POST --data "$1" http://localhost/
}

focused_on() {
	picker_status | jq -e --arg pane "$1" '.current.text | startswith($pane + "\t")' >/dev/null
}

reordered() {
	picker_status | jq -e --arg pane "$SECOND" '.matches[0].text | startswith($pane + "\t")' >/dev/null
}

@test "picker tracks the selected pane when a state change reorders rows" {
	picker_action 'pos(2)'
	wait_for focused_on "$SECOND"
	tmx set-option -p -t "$SECOND" @agent_state blocked
	wait_for reordered
	focused_on "$SECOND"
	# Another refresh must not move it back to the old list position.
	picker_action "reload('$TAP' __list)"
	wait_for focused_on "$SECOND"
}

@test "picker preserves selection when the task changes" {
	picker_action 'pos(2)'
	wait_for focused_on "$SECOND"
	tmx set-option -p -t "$SECOND" @agent_task 'updated task'
	wait_for task_updated
	focused_on "$SECOND"
}

task_updated() {
	picker_status | jq -e '.current.text | contains("updated task")' >/dev/null
}

@test "window deletion waits for confirmation and can be cancelled" {
	picker_action 'pos(2)'
	wait_for focused_on "$SECOND"
	tmx send-keys -t "$PICKER" C-w
	wait_for confirmation_visible
	tmx display-message -p -t "$SECOND" '#{pane_id}'
	tmx send-keys -t "$PICKER" n Enter
	wait_for focused_on "$SECOND"
	tmx display-message -p -t "$SECOND" '#{pane_id}'
}

@test "window confirmation keeps its original target across state changes" {
	picker_action 'pos(2)'
	wait_for focused_on "$SECOND"
	tmx send-keys -t "$PICKER" C-w
	wait_for confirmation_visible
	tmx set-option -p -t "$SECOND" @agent_state blocked
	tmx send-keys -t "$PICKER" y Enter
	wait_for second_gone
	[ "$(tmx display-message -p -t "$TMUX_PANE" '#{pane_id}')" = "$TMUX_PANE" ]
}

second_gone() {
	local panes
	panes="$(tmx list-panes -a -F '#{pane_id}')" || return 1
	! grep -Fxq "$SECOND" <<<"$panes"
}

confirmation_visible() {
	tmx capture-pane -p -t "$PICKER" | grep -q 'Kill window containing'
}
