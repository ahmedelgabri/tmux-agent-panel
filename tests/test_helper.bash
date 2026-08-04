# Shared setup for tap's bats E2E tests. Everything runs against a scratch
# tmux server on a private socket so the user's real server is never touched.

TAP="${BATS_TEST_DIRNAME}/../tap"
TMUX_SOCKET="tap-e2e-$$"

tmx() {
	tmux -L "$TMUX_SOCKET" -f /dev/null "$@"
}

start_server() {
	tmx new-session -d -s main -x 200 -y 50 'sleep 300'
	# TMUX makes the tap binary (and the tmux CLI it shells out to) talk to
	# the scratch server; TMUX_PANE targets its first pane, mirroring how
	# agent hooks inherit both from their parent.
	TMUX="$(tmx display-message -p '#{socket_path}'),0,0"
	TMUX_PANE="$(tmx display-message -p '#{pane_id}')"
	export TMUX TMUX_PANE
}

stop_server() {
	tmx kill-server 2>/dev/null || true
}
