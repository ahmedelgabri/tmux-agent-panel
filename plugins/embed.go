// Package plugins embeds the plugin hook definitions so `tap install`
// writes the exact hook sets the Claude Code and Codex plugins ship — one
// source of truth for both install channels.
package plugins

import _ "embed"

//go:embed tap-claude/hooks/hooks.json
var ClaudeHooks []byte

//go:embed tap-codex/hooks/hooks.json
var CodexHooks []byte
