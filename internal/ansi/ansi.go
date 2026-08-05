// Package ansi holds the escape codes tap renders with, in one place so the
// picker and the CLI share a palette. Named ANSI slots (not RGB) so output
// follows the terminal theme like the rest of a tmux setup.
package ansi

const (
	Reset    = "\033[0m"
	Bold     = "\033[1m"
	Gray     = "\033[90m"
	Red      = "\033[31m"
	Green    = "\033[32m"
	Yellow   = "\033[33m"
	BoldBlue = "\033[1;34m"
	Magenta  = "\033[35m"
	Cyan     = "\033[36m"
)
