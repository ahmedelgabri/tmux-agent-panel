// Hook merging for the JSON hook configs (Claude Code settings.json, Codex
// hooks.json — same schema). Entries are keyed on the "tap state" marker in
// their command string, which makes install idempotent (remove ours, then
// re-add) and uninstall surgical (everything else is preserved).
//
// Round-tripping through encoding/json normalizes key order and indentation;
// a backup is written next to the file before the first modification.
package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Marker identifies hook entries owned by tap inside foreign config files.
const Marker = "tap state"

// hookEntry is one command hook in the Claude/Codex schema.
func hookEntry(self, stateArgs string) map[string]any {
	return map[string]any{
		"type":    "command",
		"command": fmt.Sprintf("%s state %s", self, stateArgs),
		"async":   true,
	}
}

// installHooks merges tap's hook entries into the JSON file at path,
// creating the file if it doesn't exist. events maps hook event names to
// `tap state` arguments.
func installHooks(path, self string, events map[string]string) error {
	if err := checkWritable(path); err != nil {
		return err
	}

	root := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("%s: not valid JSON, refusing to modify: %w", path, err)
		}
		if err := backup(path, data); err != nil {
			return err
		}
	case os.IsNotExist(err):
		// A fresh file gets created below.
	default:
		return err
	}

	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}

	for event, stateArgs := range events {
		groups, _ := hooks[event].([]any)
		groups = removeMarked(groups)
		// tap gets its own matcher group so removal never has to reason
		// about entries sharing a group with user hooks.
		groups = append(groups, map[string]any{
			"hooks": []any{hookEntry(self, stateArgs)},
		})
		hooks[event] = groups
	}

	return writeJSON(path, root)
}

// uninstallHooks removes every tap-owned entry from the file. Missing file
// means nothing to do.
func uninstallHooks(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := checkWritable(path); err != nil {
		return err
	}

	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("%s: not valid JSON, refusing to modify: %w", path, err)
	}
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return nil
	}
	if err := backup(path, data); err != nil {
		return err
	}

	for event, v := range hooks {
		groups, _ := v.([]any)
		groups = removeMarked(groups)
		if len(groups) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = groups
		}
	}

	return writeJSON(path, root)
}

// hooksInstalled reports whether the file contains any tap-owned entry.
func hooksInstalled(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), Marker)
}

// removeMarked drops tap-owned entries from each matcher group, and drops
// groups that only contained tap entries. Groups untouched by tap pass
// through unchanged.
func removeMarked(groups []any) []any {
	var out []any
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			out = append(out, g)
			continue
		}
		entries, ok := group["hooks"].([]any)
		if !ok {
			out = append(out, g)
			continue
		}
		var kept []any
		for _, e := range entries {
			entry, ok := e.(map[string]any)
			if ok {
				if cmd, _ := entry["command"].(string); strings.Contains(cmd, Marker) {
					continue
				}
			}
			kept = append(kept, e)
		}
		if len(kept) == 0 && len(entries) > 0 && len(kept) != len(entries) {
			continue
		}
		group["hooks"] = kept
		out = append(out, group)
	}
	return out
}

// checkWritable refuses to touch files managed by Nix/Home Manager — those
// must be wired declaratively, and editing a store symlink either fails or
// silently diverges on the next rebuild.
func checkWritable(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil && strings.Contains(resolved, "/nix/store/") {
		return fmt.Errorf("%s is managed by Nix/Home Manager; add the hooks to your configuration instead", path)
	}
	if _, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("%s is not writable: %w", path, err)
		}
		f.Close()
	}
	return nil
}

func backup(path string, data []byte) error {
	return os.WriteFile(path+".tap.bak", data, 0o600)
}

func writeJSON(path string, root map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(root, "", "\t")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tap-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
