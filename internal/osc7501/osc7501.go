// Package osc7501 reads agent state from Program Status Protocol (OSC 7501)
// records. Agents send these records to their terminal; a host that keeps
// them (Rex) hands them to tap, so such hosts need no hooks. tmux keeps no
// records, so agents there report through tap's hooks instead. `tap state`
// stays a no-op outside tmux: writing reports from the hooks as well would
// overwrite the agents' own, e.g. replace Claude's `done` with `idle` at
// Stop.
// https://www.superlogical.com/rex/docs/build/program-status
package osc7501

import "github.com/ahmedelgabri/tmux-agent-panel/internal/agents"

// Record is a pane's root record, keyed as in the protocol.
type Record struct {
	App   string `json:"app"`
	State string `json:"state"`
	Kind  string `json:"kind"`
	Msg   string `json:"msg"`
}

// Agent returns the agent the record's app names. Any program can report
// status; only agents' records become agent state, or a build tool's
// record would read as an orphaned agent.
func (r Record) Agent() (agents.Agent, bool) {
	return agents.ForApp(r.App)
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
