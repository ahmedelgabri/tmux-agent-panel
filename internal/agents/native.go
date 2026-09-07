package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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

// Enablement/scopes: https://code.claude.com/docs/en/plugins-reference.
// The installed_plugins.json v2 registry layout was observed on a real install;
// it is not a documented API contract.
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

// Cache paths and active-version precedence follow plugin_base_root,
// active_plugin_version, and compare_plugin_versions in:
// https://github.com/openai/codex/blob/2230d644/codex-rs/core-plugins/src/store.rs
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
		Plugins map[string]struct{ Enabled *bool }
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("%s: %w", configPath, err)
	}
	plugin, configured := config.Plugins["tap-codex@tmux-agent-panel"]
	// Codex defaults a configured plugin to enabled when the field is omitted.
	if !configured || (plugin.Enabled != nil && !*plugin.Enabled) {
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

// User-scope package paths and filters follow:
// https://github.com/earendil-works/pi/blob/v0.85.1/packages/coding-agent/src/core/package-manager.ts
func piNative() (string, error) {
	extension, err := piExtensionPath()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(filepath.Dir(extension))
	var config struct {
		Packages   []json.RawMessage `json:"packages"`
		NpmCommand []string          `json:"npmCommand"`
	}
	if err := readConfig(filepath.Join(dir, "settings.json"), &config); err != nil {
		return "", err
	}
	for _, raw := range config.Packages {
		var source string
		var filtered struct {
			Source     string   `json:"source"`
			Extensions []string `json:"extensions"`
			Autoload   *bool    `json:"autoload"`
		}
		if json.Unmarshal(raw, &source) != nil {
			if err := json.Unmarshal(raw, &filtered); err != nil {
				return "", fmt.Errorf("pi package settings: %w", err)
			}
			source = filtered.Source
		}
		autoload := filtered.Autoload == nil || *filtered.Autoload
		// Empty filters need no path resolution or package-manager lookup.
		if len(filtered.Extensions) == 0 && (filtered.Extensions != nil || !autoload) {
			continue
		}
		var root string
		switch {
		case piGitSource.MatchString(source) && (strings.HasPrefix(source, "git:") || strings.Contains(source, "://")):
			root = filepath.Join(dir, "git", "github.com", "ahmedelgabri", "tmux-agent-panel")
		case strings.HasPrefix(source, "npm:"):
			spec := strings.TrimSpace(strings.TrimPrefix(source, "npm:"))
			if spec != "pi-tmux-agent-panel" && !strings.HasPrefix(spec, "pi-tmux-agent-panel@") {
				continue
			}
			root = piNpmRoot(dir, config.NpmCommand)
		default:
			root, err = piLocalPath(dir, source)
			if err != nil {
				return "", fmt.Errorf("pi package %q: %w", source, err)
			}
			if root == "" {
				continue
			}
			var manifest struct{ Name string }
			if readConfig(filepath.Join(root, "package.json"), &manifest) != nil || manifest.Name != "pi-tmux-agent-panel" {
				continue
			}
		}
		root, err = filepath.Abs(root)
		if err != nil {
			return "", err
		}
		if !piExtensionEnabled(root, filtered.Extensions, autoload) {
			continue
		}
		path := filepath.Join(root, piPackageExtension)
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
