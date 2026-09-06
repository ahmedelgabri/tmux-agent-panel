// Hook merging for the JSON hook configs (Claude Code settings.json, Codex
// hooks.json — same schema). Entries are identified by a direct `tap state`
// invocation, which makes install idempotent (remove ours, then
// re-add) and uninstall surgical (everything else is preserved).
//
// Edits are spliced into the original bytes with sjson/gjson so user
// content keeps its exact key order, indentation, and escapes; a backup is
// written next to the file before the first modification.
package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/junegunn/go-shellwords"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Marker is the prefix of the canonical hook commands.
const Marker = "tap state"

// ownsCommand accepts direct invocations, including quoted absolute paths
// from legacy installs. Mentions in another command and compound scripts
// belong to the user; removing those could discard unrelated work.
func ownsCommand(command string) bool {
	if strings.ContainsAny(command, "\r\n") {
		return false
	}
	p := &shellwords.Parser{}
	args, err := p.Parse(command)
	if err != nil || p.Position >= 0 {
		return false
	}
	if len(args) > 0 && args[0] == "exec" {
		args = args[1:]
	}
	return len(args) >= 3 && filepath.Base(args[0]) == "tap" && args[1] == "state"
}

// jsonHookFuncs wires the shared install flow for agents whose integration
// is a JSON hooks file (Claude, Codex): resolve the config path, then
// merge/strip/detect tap's embedded hook set.
func jsonHookFuncs(pathFn func() (string, error), hooks []byte) (install, uninstall func() error, installed, current func() bool) {
	install = func() error {
		path, err := pathFn()
		if err != nil {
			return err
		}
		return installHooks(path, hooks)
	}
	uninstall = func() error {
		path, err := pathFn()
		if err != nil {
			return err
		}
		return uninstallHooks(path)
	}
	installed = func() bool {
		path, err := pathFn()
		return err == nil && hooksInstalled(path)
	}
	current = func() bool {
		path, err := pathFn()
		return err != nil || hooksCurrent(path, hooks)
	}
	return install, uninstall, installed, current
}

// pluginHooks is the shape of an embedded plugin hooks.json.
type pluginHooks struct {
	Hooks map[string][]any `json:"hooks"`
}

// resolveTarget follows symlinks so writes land in the linked-to file.
// Renaming over the symlink path itself would replace the link with a
// regular file, silently detaching e.g. a dotfiles-managed settings.json.
func resolveTarget(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// installHooks splices the hook groups from an embedded plugin hooks.json
// verbatim into the JSON file at path, creating the file if it doesn't
// exist. Existing content is preserved byte-for-byte — round-tripping the
// document through Go maps would sort every key and re-indent the file.
// Commands invoke `tap` from PATH — identical to the plugin channel,
// and immune to the binary moving (Nix store paths change every rebuild).
func installHooks(path string, hooksJSON []byte) error {
	var plugin pluginHooks
	if err := json.Unmarshal(hooksJSON, &plugin); err != nil {
		return fmt.Errorf("parsing embedded plugin hooks: %w", err)
	}

	if err := checkWritable(path); err != nil {
		return err
	}
	path = resolveTarget(path)

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		// A fresh file has no formatting to preserve; write the canonical
		// tab-indented layout.
		return writeJSON(path, map[string]any{"hooks": plugin.Hooks})
	default:
		return err
	}

	if !gjson.ValidBytes(data) || !gjson.ParseBytes(data).IsObject() {
		return fmt.Errorf("%s: not a valid JSON object, refusing to modify", path)
	}
	if err := backup(path, data); err != nil {
		return err
	}

	// Stripping every event (not just the embedded set's) migrates entries
	// out of events a newer hook set no longer contains; without it a stale
	// entry would linger and doctor would report the install outdated with
	// no way to converge short of uninstalling.
	doc := stripAllMarked(string(data))
	var spliceErr error
	gjson.GetBytes(hooksJSON, "hooks").ForEach(func(event, groups gjson.Result) bool {
		eventPath := "hooks." + event.String()
		for _, g := range groups.Array() {
			// Compacting keeps the embedded key order but drops its
			// indentation: a multi-line splice would not survive the
			// remove-then-append cycle byte-exact, compact one-liners do.
			var compact bytes.Buffer
			if spliceErr = json.Compact(&compact, []byte(g.Raw)); spliceErr != nil {
				return false
			}
			if doc, spliceErr = sjson.SetRaw(doc, eventPath+".-1", compact.String()); spliceErr != nil {
				return false
			}
		}
		return true
	})
	if spliceErr != nil {
		return spliceErr
	}

	return writeFileAtomic(path, []byte(doc))
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
	path = resolveTarget(path)

	if !gjson.ValidBytes(data) {
		return fmt.Errorf("%s: not valid JSON, refusing to modify", path)
	}
	hooks := gjson.GetBytes(data, "hooks")
	if !hooks.IsObject() {
		return nil
	}
	if err := backup(path, data); err != nil {
		return err
	}

	return writeFileAtomic(path, []byte(stripAllMarked(string(data))))
}

// stripAllMarked removes tap-owned entries under every event in the
// document. tap's groups never mix with user hooks, so removal stays
// surgical; untouched events keep their exact bytes.
func stripAllMarked(doc string) string {
	hooks := gjson.Get(doc, "hooks")
	if !hooks.IsObject() {
		return doc
	}
	var events []string
	hooks.ForEach(func(event, _ gjson.Result) bool {
		events = append(events, event.String())
		return true
	})
	for _, event := range events {
		eventPath := "hooks." + event
		stripped := stripMarked(doc, eventPath)
		if stripped == doc {
			continue
		}
		doc = stripped
		// An event emptied by the removal is dropped, matching install,
		// which only ever adds whole groups.
		if len(gjson.Get(doc, eventPath).Array()) == 0 {
			doc, _ = sjson.Delete(doc, eventPath)
		}
	}
	return doc
}

// hooksInstalled reports whether the file contains any tap-owned entry.
func hooksInstalled(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var root pluginHooks
	return json.Unmarshal(data, &root) == nil && len(markedCommands(root.Hooks)) > 0
}

// hooksCurrent reports whether the tap-owned entries in the file match the
// embedded hook set, catching stale installs whose commands predate a flag
// (e.g. --agent). Only meaningful when hooks are installed; a missing or
// unreadable file counts as current so "not installed" stays the only
// finding.
func hooksCurrent(path string, hooksJSON []byte) bool {
	var plugin pluginHooks
	if err := json.Unmarshal(hooksJSON, &plugin); err != nil {
		return true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var root pluginHooks
	if err := json.Unmarshal(data, &root); err != nil {
		return false
	}
	return slices.Equal(markedCommands(plugin.Hooks), markedCommands(root.Hooks))
}

// markedCommands lists the tap-owned (event, command) pairs, sorted so two
// hook sets compare structurally regardless of group layout.
func markedCommands(hooks map[string][]any) []string {
	var out []string
	for event, groups := range hooks {
		for _, g := range groups {
			group, ok := g.(map[string]any)
			if !ok {
				continue
			}
			entries, _ := group["hooks"].([]any)
			for _, e := range entries {
				entry, ok := e.(map[string]any)
				if !ok {
					continue
				}
				if cmd, _ := entry["command"].(string); ownsCommand(cmd) {
					out = append(out, event+"\t"+cmd)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// stripMarked deletes tap-owned entries under the given event path, and
// drops groups that only contained tap entries. Groups untouched by tap
// keep their exact bytes. Deletions walk backwards so the indexes of
// earlier siblings stay valid.
func stripMarked(doc, eventPath string) string {
	groups := gjson.Get(doc, eventPath).Array()
	for gi := len(groups) - 1; gi >= 0; gi-- {
		entries := groups[gi].Get("hooks").Array()
		marked := 0
		for _, e := range entries {
			if ownsCommand(e.Get("command").String()) {
				marked++
			}
		}
		if marked == 0 {
			continue
		}
		if marked == len(entries) {
			doc, _ = sjson.Delete(doc, fmt.Sprintf("%s.%d", eventPath, gi))
			continue
		}
		for ei := len(entries) - 1; ei >= 0; ei-- {
			if ownsCommand(entries[ei].Get("command").String()) {
				doc, _ = sjson.Delete(doc, fmt.Sprintf("%s.%d.hooks.%d", eventPath, gi, ei))
			}
		}
	}
	return doc
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
	path += ".tap.bak"
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

func writeJSON(path string, root map[string]any) error {
	out, err := json.MarshalIndent(root, "", "\t")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(out, '\n'))
}

func writeFileAtomic(path string, out []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
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
