module github.com/ahmedelgabri/tmux-agent-panel

go 1.26.4

// Avoid fzf aborting after 100 interrupted terminal waits: https://github.com/junegunn/fzf/issues/4917.
replace github.com/junegunn/fzf => ./third_party/fzf

require (
	github.com/benjaminnkem/minimatch-go v0.1.0
	github.com/junegunn/fzf v0.74.2
	github.com/junegunn/go-shellwords v0.0.0-20250127100254-2aa3b3277741
	github.com/nlnwa/whatwg-url v0.6.2
	github.com/pelletier/go-toml/v2 v2.4.3
	github.com/rivo/uniseg v0.4.7
	github.com/spf13/cobra v1.10.2
	github.com/tidwall/gjson v1.14.2
	github.com/tidwall/sjson v1.2.5
)

require (
	github.com/bits-and-blooms/bitset v1.20.0 // indirect
	github.com/charlievieth/fastwalk v1.0.14 // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/gdamore/tcell/v2 v2.9.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/term v0.43.0 // indirect
	golang.org/x/text v0.39.0 // indirect
)
