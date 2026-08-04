package state

import (
	"strings"
	"testing"
)

func TestRouteNotification(t *testing.T) {
	cases := map[string]string{
		`{"message":"Claude needs your permission to use Bash"}`: "blocked",
		`{"message":"Permission required"}`:                      "blocked",
		`{"message":"Claude is waiting for your input"}`:         "waiting",
		`{"message":"Do you want to continue?"}`:                 "waiting",
		`{}`:                                                     "waiting",
		`not even json`:                                          "waiting",
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
