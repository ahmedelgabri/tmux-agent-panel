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
	if [ -z "${BATS_TEST_COMPLETED:-}" ]; then
		if [ -S "${FZF_SOCKET:-}" ]; then picker_status >&2 || true; fi
		if [ -n "${PICKER:-}" ]; then
			tmx display-message -p -t "$PICKER" 'dead=#{pane_dead} status=#{pane_dead_status} signal=#{pane_dead_signal} pid=#{pane_pid}' >&2 || true
			local pid
			pid="$(tmx display-message -p -t "$PICKER" '#{pane_pid}')"
			ps -p "$pid" -o pid,ppid,stat,command >&2 || true
			tmx capture-pane -p -t "$PICKER" >&2 || true
		fi
	fi
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

# fzf drops cursor and execute actions while restoring tracked selection.
# Retry only setup interactions; assertions after state changes stay passive
# so a tracking regression cannot be hidden by reselecting the expected pane.
select_second() {
	picker_action 'pos(2)' && focused_on "$SECOND"
}

open_window_confirmation() {
	if confirmation_visible; then return 0; fi
	tmx send-keys -t "$PICKER" C-w
	confirmation_visible
}

reordered() {
	picker_status | jq -e --arg pane "$SECOND" '.matches[0].text | startswith($pane + "\t")' >/dev/null
}

blocked_reload_started() {
	if [ -f "$TMUX_TEST_DIR/reload-started" ]; then return 0; fi
	picker_action "reload(touch '$TMUX_TEST_DIR/reload-started'; while [ -f '$TMUX_TEST_DIR/reload-started' ] && [ ! -f '$TMUX_TEST_DIR/reload-release' ]; do sleep 0.01; done; '$TAP' __list)"
	return 1
}

@test "picker setup handles cursor actions dropped during a reload" {
	wait_for blocked_reload_started
	run select_second
	[ "$status" -ne 0 ]
	focused_on "$TMUX_PANE"
	touch "$TMUX_TEST_DIR/reload-release"
	wait_for select_second
}

@test "picker tracks the selected pane when a state change reorders rows" {
	wait_for select_second
	tmx set-option -p -t "$SECOND" @agent_state blocked
	wait_for reordered
	focused_on "$SECOND"
	# Another refresh must not move it back to the old list position.
	picker_action "reload('$TAP' __list)"
	wait_for focused_on "$SECOND"
}

@test "cached views include live changes to plain panes" {
	wait_for show_all_panes
	local plain
	plain="$(tmx new-window -d -t main -n plain-before -P -F '#{pane_id}' 'sleep 300')"
	wait_for row_contains "$plain" plain-before
	tmx rename-window -t "$plain" plain-updated
	wait_for row_contains "$plain" plain-updated
	wait_for show_agent_panes
	picker_status | jq -e '.matchCount == 2' >/dev/null
}

show_all_panes() {
	if row_contains "$PICKER" picker; then return 0; fi
	tmx send-keys -t "$PICKER" C-a
	row_contains "$PICKER" picker
}

show_agent_panes() {
	if picker_status | jq -e '.matchCount == 2' >/dev/null; then return 0; fi
	tmx send-keys -t "$PICKER" C-a
	picker_status | jq -e '.matchCount == 2' >/dev/null
}

row_contains() {
	picker_status | jq -e --arg pane "$1" --arg text "$2" '.matches[] | select(.text | startswith($pane + "\t")) | .text | contains($text)' >/dev/null
}

@test "typing stays intact while agent state changes" {
	local query='second second second second' i
	for ((i = 0; i < ${#query}; i++)); do
		# Send each key once: retrying would hide dropped input.
		tmx send-keys -t "$PICKER" -l "${query:i:1}"
		if [ "$i" -eq 6 ]; then
			tmx set-option -p -t "$SECOND" @agent_state blocked
		fi
		sleep 0.05
	done
	picker_status | jq -e --arg query "$query" '.query == $query' >/dev/null
	wait_for blocked_row_visible
}

blocked_row_visible() {
	picker_status | jq -e --arg pane "$SECOND" '.matches[] | select(.text | startswith($pane + "\t")) | .text | contains("▲")' >/dev/null
}

@test "picker preserves selection when the task changes" {
	wait_for select_second
	tmx set-option -p -t "$SECOND" @agent_task 'updated task'
	wait_for task_updated
	focused_on "$SECOND"
}

task_updated() {
	picker_status | jq -e '.current.text | contains("updated task")' >/dev/null
}

@test "window deletion waits for confirmation and can be cancelled" {
	wait_for select_second
	wait_for open_window_confirmation
	tmx display-message -p -t "$SECOND" '#{pane_id}'
	tmx send-keys -t "$PICKER" n Enter
	wait_for focused_on "$SECOND"
	tmx display-message -p -t "$SECOND" '#{pane_id}'
}

@test "window confirmation keeps its original target across state changes" {
	wait_for select_second
	wait_for open_window_confirmation
	tmx set-option -p -t "$SECOND" @agent_state blocked
	tmx send-keys -t "$PICKER" y Enter
	wait_for second_gone
	[ "$(tmx display-message -p -t "$TMUX_PANE" '#{pane_id}')" = "$TMUX_PANE" ]
}

@test "picker accepts the selected pane and exits" {
	# A control-mode client lets switch-client succeed without a real terminal.
	mkfifo "$TMUX_TEST_DIR/client.in"
	(
		exec 3<>"$TMUX_TEST_DIR/client.in"
		tmx -C attach-session -t main <&3 >"$TMUX_TEST_DIR/client.log"
	) &
	wait_for client_attached
	tmx set-option -w -t "$PICKER" remain-on-exit on
	wait_for select_second
	wait_for finish_picker accept
	[ "$(tmx display-message -p -t "$PICKER" '#{pane_dead_status}')" = 0 ]
	[ "$(tmx list-clients -F '#{pane_id}')" = "$SECOND" ]
}

@test "picker cancellation exits without selecting a pane" {
	tmx set-option -w -t "$PICKER" remain-on-exit on
	wait_for finish_picker abort
	[ "$(tmx display-message -p -t "$PICKER" '#{pane_dead_status}')" = 0 ]
}

@test "picker exit check distinguishes pty closure from process completion" {
	# Guard the pane_dead/exit-status race from Linux CI run 34058266359.
	# These simulated replies fail the regression with a pane_dead-only helper.
	(
		# Keep the simulated replies local so teardown still uses the real tmux.
		tmx() {
			case "$*" in
			*'#{pane_dead}') echo 1 ;;
			*) printf '%s\n' "$exit_status" ;;
			esac
		}
		exit_status=''
		run picker_exited
		[ "$status" -ne 0 ]
		exit_status=0
		picker_exited
		exit_status=TERM
		picker_exited
	)
}

client_attached() {
	tmx list-clients -F '#{client_control_mode}' | grep -qx 1
}

picker_exited() {
	# A closed pty does not mean tmux has collected the process exit status.
	[ -n "$(tmx display-message -p -t "$PICKER" '#{pane_dead_status}#{pane_dead_signal}')" ]
}

finish_picker() {
	if picker_exited; then return 0; fi
	picker_action "$1" && picker_exited
}

second_gone() {
	local panes
	panes="$(tmx list-panes -a -F '#{pane_id}')" || return 1
	! grep -Fxq "$SECOND" <<<"$panes"
}

confirmation_visible() {
	tmx capture-pane -p -t "$PICKER" | grep -q 'Kill window containing'
}
