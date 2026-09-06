package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/ahmedelgabri/tmux-agent-panel/plugins"
)

func readConfig(path string, dst any) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err == nil {
		err = json.Unmarshal(data, dst)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func nativeHooks(root string, hooks []byte) (string, error) {
	path := filepath.Join(root, "hooks", "hooks.json")
	if !hooksInstalled(path) {
		return "", fmt.Errorf("native plugin hooks missing or invalid (%s); reinstall through the agent", path)
	}
	if !hooksCurrent(path, hooks) {
		return "", fmt.Errorf("native plugin hooks outdated (%s); update through the agent", path)
	}
	return "native plugin installed (" + root + ")", nil
}

func claudeNative() (string, error) {
	settings, err := claudeSettingsPath()
	if err != nil {
		return "", err
	}
	var config struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err := readConfig(settings, &config); err != nil {
		return "", err
	}
	const key = "tap@tmux-agent-panel"
	if !config.EnabledPlugins[key] {
		return "", nil
	}
	var registry struct {
		Plugins map[string][]struct {
			Scope       string `json:"scope"`
			InstallPath string `json:"installPath"`
		} `json:"plugins"`
	}
	if err := readConfig(filepath.Join(filepath.Dir(settings), "plugins", "installed_plugins.json"), &registry); err != nil {
		return "", err
	}
	for _, entry := range registry.Plugins[key] {
		if entry.Scope == "user" && entry.InstallPath != "" {
			return nativeHooks(entry.InstallPath, plugins.ClaudeHooks)
		}
	}
	return "", fmt.Errorf("native plugin %s enabled but not installed; run `claude plugin install %s`", key, key)
}

func codexNative() (string, error) {
	hooks, err := codexHooksPath()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(filepath.Dir(hooks), "config.toml")
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var config struct {
		Plugins map[string]struct{ Enabled bool }
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("%s: %w", configPath, err)
	}
	if !config.Plugins["tap-codex@tmux-agent-panel"].Enabled {
		return "", nil
	}
	base := filepath.Join(filepath.Dir(hooks), "plugins", "cache", "tmux-agent-panel", "tap-codex")
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", fmt.Errorf("native plugin cache unavailable (%s); reinstall through Codex: %w", base, err)
	}
	// Codex prefers local, otherwise the greatest semantic version. tap
	// publishes dotted triples; non-version cache names compare lexically.
	version := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "local" {
			version = name
			break
		}
		a, aOK := parseVersion(version)
		b, bOK := parseVersion(name)
		newer := version < name
		if aOK && bOK {
			newer = versionLess(a, b)
		}
		if newer {
			version = name
		}
	}
	if version == "" {
		return "", fmt.Errorf("native plugin cache empty (%s); reinstall through Codex", base)
	}
	return nativeHooks(filepath.Join(base, version), plugins.CodexHooks)
}

var piGitSource = regexp.MustCompile(`^(git:)?(https?://github\.com/|ssh://git@github\.com/|git@github\.com:|github\.com/)ahmedelgabri/tmux-agent-panel(\.git)?(@[^\s]+)?$`)

func piNative() (string, error) {
	extension, err := piExtensionPath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(filepath.Dir(extension))
	var config struct {
		Packages []json.RawMessage `json:"packages"`
	}
	if err := readConfig(filepath.Join(dir, "settings.json"), &config); err != nil {
		return "", err
	}
	for _, raw := range config.Packages {
		var source string
		if json.Unmarshal(raw, &source) != nil {
			var filtered struct {
				Source     string   `json:"source"`
				Extensions []string `json:"extensions"`
				Autoload   *bool    `json:"autoload"`
			}
			if err := json.Unmarshal(raw, &filtered); err != nil {
				return "", fmt.Errorf("pi package settings: %w", err)
			}
			autoload := filtered.Autoload == nil || *filtered.Autoload
			if !piExtensionEnabled(filtered.Extensions, autoload) {
				continue
			}
			source = filtered.Source
		}
		var root string
		switch {
		case piGitSource.MatchString(source):
			root = filepath.Join(dir, "git", "github.com", "ahmedelgabri", "tmux-agent-panel")
		case source == "npm:pi-tmux-agent-panel" || strings.HasPrefix(source, "npm:pi-tmux-agent-panel@"):
			root = filepath.Join(dir, "npm", "node_modules", "pi-tmux-agent-panel")
		case filepath.IsAbs(source) || strings.HasPrefix(source, ".") || strings.HasPrefix(source, "~/"):
			root = source
			if strings.HasPrefix(root, "~/") {
				home, err := os.UserHomeDir()
				if err != nil {
					return "", err
				}
				root = filepath.Join(home, root[2:])
			} else if !filepath.IsAbs(root) {
				root = filepath.Join(dir, root)
			}
			var manifest struct{ Name string }
			if readConfig(filepath.Join(root, "package.json"), &manifest) != nil || manifest.Name != "pi-tmux-agent-panel" {
				continue
			}
		default:
			continue
		}
		path := filepath.Join(root, "extensions", "tap-agent-state.ts")
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("native pi package unavailable (%s); run `pi install %s`: %w", path, source, err)
		}
		if !bytes.Equal(data, piExtension) {
			return "", fmt.Errorf("native pi package outdated (%s); update through pi", path)
		}
		return "native package installed (" + root + ")", nil
	}
	return "", nil
}

func piExtensionEnabled(filters []string, autoload bool) bool {
	if filters == nil {
		return autoload
	}
	if len(filters) == 0 {
		return false
	}
	const extension = "extensions/tap-agent-state.ts"
	matches := func(pattern string) bool {
		pattern = strings.TrimPrefix(strings.TrimPrefix(pattern, "./"), "**/")
		full, _ := path.Match(pattern, extension)
		base, _ := path.Match(pattern, path.Base(extension))
		return full || base
	}
	exact := func(pattern string) bool { return strings.TrimPrefix(pattern, "./") == extension }
	var included, excluded, forceIncluded, hasIncludes bool
	for _, filter := range filters {
		switch {
		case strings.HasPrefix(filter, "-"):
			// Exact exclusions override every other filter.
			if exact(filter[1:]) {
				return false
			}
		case strings.HasPrefix(filter, "+"):
			forceIncluded = forceIncluded || exact(filter[1:])
		case strings.HasPrefix(filter, "!"):
			excluded = excluded || matches(filter[1:])
		default:
			hasIncludes = true
			included = included || matches(filter)
		}
	}
	if forceIncluded {
		return true
	}
	if excluded {
		return false
	}
	if hasIncludes {
		return included
	}
	return autoload
}
