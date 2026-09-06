# Shared setup for tap's bats E2E tests. Everything runs against a scratch
# tmux server on a private socket so the user's real server is never touched.

TAP="${BATS_TEST_DIRNAME}/../tap"
tmx() {
	tmux -S "${TMUX_SOCKET:?private server not started}" -f /dev/null "$@"
}

start_server() {
	# Short paths also fit macOS's Unix socket limit.
	TMUX_TEST_DIR="$(mktemp -d /tmp/tap-e2e.XXXXXX)"
	TMUX_SOCKET="$TMUX_TEST_DIR/tmux.sock"
	tmx new-session -d -s main -x 200 -y 50 'sleep 300'
	# TMUX makes the tap binary (and the tmux CLI it shells out to) talk to
	# the scratch server; TMUX_PANE targets its first pane, mirroring how
	# agent hooks inherit both from their parent.
	TMUX="$(tmx display-message -p '#{socket_path}'),0,0"
	TMUX_PANE="$(tmx display-message -p '#{pane_id}')"
	export TMUX TMUX_PANE
}

stop_server() {
	if [ -n "${TMUX_TEST_DIR:-}" ]; then
		tmx kill-server 2>/dev/null || true
		rm -rf "$TMUX_TEST_DIR"
	fi
}

# Polling avoids fixed startup sleeps on slower CI runners.
wait_for() {
	local attempt
	for ((attempt = 0; attempt < 100; attempt++)); do
		if "$@"; then return 0; fi
		sleep 0.05
	done
	echo "timed out: $*" >&2
	return 1
}

# Pipes a Notification hook payload through `tap state notification` and
# asserts the pane's @agent_state ends up as expected.
assert_notification_state() {
	run bash -c "echo '$1' | '$TAP' state notification"
	[ "$status" -eq 0 ]
	run tmx show-options -pv -t "$TMUX_PANE" @agent_state
	[ "$output" = "$2" ]
}
