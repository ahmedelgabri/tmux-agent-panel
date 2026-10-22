package state

import "testing"

func TestFromProgramStatus(t *testing.T) {
	cases := map[[2]string]string{
		{"working", ""}:           "running",
		{"blocked", "permission"}: "blocked",
		{"blocked", "auth"}:       "blocked",
		{"blocked", ""}:           "blocked",
		{"blocked", "question"}:   "waiting",
		{"idle", ""}:              "idle",
		{"done", ""}:              "idle",
		{"error", ""}:             "idle",
		{"", ""}:                  "idle",
	}
	for in, want := range cases {
		if got := FromProgramStatus(in[0], in[1]); got != want {
			t.Errorf("FromProgramStatus(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}
