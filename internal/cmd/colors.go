package cmd

import "os"

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiGray   = "\033[90m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
)

func colorize(color, s string) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return color + s + ansiReset
}

func stateColor(st string) string {
	switch st {
	case "running":
		return ansiGreen
	case "blocked":
		return ansiRed
	case "waiting":
		return ansiYellow
	}
	return ansiGray
}

// stdoutIsTTY separates interactive use from hook invocations: hooks get
// silent plumbing, humans get feedback.
func stdoutIsTTY() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
