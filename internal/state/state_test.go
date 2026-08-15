package state

import (
	"strings"
	"testing"
)

func TestRouteNotification(t *testing.T) {
	cases := map[string]string{
		// notification_type routing, independent of message wording.
		`{"notification_type":"permission_prompt","message":"Claude needs your permission"}`:               "blocked",
		`{"notification_type":"permission_prompt","message":"localized wording"}`:                          "blocked",
		`{"notification_type":"worker_permission_prompt","message":"my-worker needs permission for Bash"}`: "blocked",
		`{"notification_type":"elicitation_dialog","message":"MCP server needs your input"}`:               "waiting",
		`{"notification_type":"elicitation_url_dialog"}`:                                                   "waiting",
		`{"notification_type":"agent_needs_input"}`:                                                        "waiting",
		`{"notification_type":"idle_prompt","message":"Claude is waiting for your input"}`:                 "idle",
		`{"notification_type":"auth_success"}`:                                                             "",
		`{"notification_type":"elicitation_complete"}`:                                                     "",
		`{"notification_type":"elicitation_response"}`:                                                     "",
		`{"notification_type":"agent_completed"}`:                                                          "",
		// Unknown or absent type falls back to the message-text heuristic.
		`{"notification_type":"some_future_type","message":"Permission required"}`: "blocked",
		`{"message":"Claude needs your permission to use Bash"}`:                   "blocked",
		`{"message":"Permission required"}`:                                        "blocked",
		`{"message":"Claude is waiting for your input"}`:                           "waiting",
		`{"message":"Do you want to continue?"}`:                                   "waiting",
		`{}`:                                                                       "waiting",
		`not even json`:                                                            "waiting",
	}
	for payload, want := range cases {
		if got := RouteNotification([]byte(payload)); got != want {
			t.Errorf("RouteNotification(%s) = %q, want %q", payload, got, want)
		}
	}
}

func TestTaskFromPrompt(t *testing.T) {
	if got := TaskFromPrompt([]byte(`{"prompt":"fix the bug\nand more"}`)); got != "fix the bug" {
		t.Errorf("first line only, got %q", got)
	}
	if got := TaskFromPrompt([]byte(`{"prompt":"a\tb"}`)); got != "a b" {
		t.Errorf("tabs become spaces, got %q", got)
	}
	long := strings.Repeat("é", 100)
	if got := TaskFromPrompt([]byte(`{"prompt":"` + long + `"}`)); len([]rune(got)) != TaskMaxLength {
		t.Errorf("truncation should count runes, got %d", len([]rune(got)))
	}
	if got := TaskFromPrompt([]byte(`{}`)); got != "" {
		t.Errorf("missing prompt yields empty task, got %q", got)
	}
}
