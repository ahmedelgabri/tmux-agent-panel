package osc7501

import "testing"

func TestTapState(t *testing.T) {
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
		if got := (Record{State: in[0], Kind: in[1]}).TapState(); got != want {
			t.Errorf("TapState(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}
