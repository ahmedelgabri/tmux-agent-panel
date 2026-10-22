package state

// Rex has no pane options. Claude Code, Codex, and pi report their own state
// there with the Program Status Protocol (OSC 7501), so `tap state` stays a
// no-op in Rex and the picker reads those reports instead. Writing reports
// from tap's hooks as well would overwrite the agents' own, e.g. replace
// Claude's `done` with `idle` at Stop.
// https://www.superlogical.com/rex/docs/build/program-status

// FromProgramStatus maps a program status record to a tap state. done and
// error both leave the agent waiting for the next prompt, so they read as
// idle, like unknown states.
func FromProgramStatus(st, kind string) string {
	switch st {
	case "working":
		return "running"
	case "blocked":
		if kind == "question" {
			return "waiting"
		}
		return "blocked"
	}
	return "idle"
}
