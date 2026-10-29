// Package osc7501 reads agent state from Program Status Protocol (OSC 7501)
// records. Agents send these records to their terminal; a host that keeps
// them (Rex) hands them to tap, so such hosts need no hooks. tmux keeps no
// records, so agents there report through tap's hooks instead. `tap state`
// stays a no-op outside tmux: writing reports from the hooks as well would
// overwrite the agents' own, e.g. replace Claude's `done` with `idle` at
// Stop.
// https://www.superlogical.com/rex/docs/build/program-status
package osc7501

// Record is a pane's root record, keyed as in the protocol. App picks the
// pane's agent (see agents.ForApp); Title, a human-readable label, names it.
type Record struct {
	App   string `json:"app"`
	Title string `json:"title"`
	State string `json:"state"`
	Kind  string `json:"kind"`
	Msg   string `json:"msg"`
}

// TapState maps the record to a tap state. done and error both leave the
// agent waiting for the next prompt, so they read as idle, like unknown
// states.
func (r Record) TapState() string {
	switch r.State {
	case "working":
		return "running"
	case "blocked":
		if r.Kind == "question" {
			return "waiting"
		}
		return "blocked"
	}
	return "idle"
}
