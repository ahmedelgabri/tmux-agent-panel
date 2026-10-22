package picker

import "testing"

func TestExpandCursorForward(t *testing.T) {
	in := "\x1b[0m\x1b[38;2;255;255;255m⏺\x1b[1C\x1b[0mI'll\x1b[1Csurvey\x1b[CA\x1b[0CB\x1b[3C|"
	want := "\x1b[0m\x1b[38;2;255;255;255m⏺ \x1b[0mI'll survey A B   |"
	if got := expandCursorForward(in); got != want {
		t.Errorf("expandCursorForward() = %q, want %q", got, want)
	}
}
