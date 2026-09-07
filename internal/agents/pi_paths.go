package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Local path normalization follows Pi's user-settings base directory:
// https://github.com/earendil-works/pi/blob/v0.85.1/packages/coding-agent/src/utils/paths.ts
func piLocalPath(dir, source string) (string, error) {
	if !piLocalSource(source) {
		return "", nil
	}
	root := trimPiSpace(source)
	switch {
	case root == "~" || strings.HasPrefix(root, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(root, "~"), "/"))
	case strings.HasPrefix(root, "file://"):
		u, err := url.Parse(root)
		if err != nil {
			return "", err
		}
		// Node's fileURLToPath rejects remote hosts and encoded slashes on Unix.
		if u.User != nil || (u.Host != "" && !strings.EqualFold(u.Host, "localhost")) || strings.Contains(strings.ToLower(u.EscapedPath()), "%2f") {
			return "", fmt.Errorf("invalid local file URL %q", source)
		}
		root = u.Path
		if root == "" {
			root = "/"
		}
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(dir, root)
	}
	return filepath.Abs(root)
}

func piLocalSource(source string) bool {
	source = trimPiSpace(source)
	for _, prefix := range []string{"npm:", "git:", "github:", "http:", "https:", "ssh:"} {
		if strings.HasPrefix(source, prefix) {
			return false
		}
	}
	return true
}

// Match getNpmInstallPath/getLegacyGlobalNpmInstallPath, including wrappers:
// https://github.com/earendil-works/pi/blob/v0.85.1/packages/coding-agent/src/core/package-manager.ts
func piNpmRoot(dir string, command []string) string {
	const name = "pi-tmux-agent-panel"
	managed := filepath.Join(dir, "npm", "node_modules", name)
	if _, err := os.Stat(managed); err == nil {
		return managed
	}
	if len(command) == 0 {
		command = []string{"npm"}
	}
	manager := command[0]
	for i, arg := range command {
		if arg == "--" {
			manager = ""
			if i+1 < len(command) {
				manager = command[i+1]
			}
		}
	}
	manager = filepath.Base(manager)
	if ext := filepath.Ext(manager); strings.EqualFold(ext, ".cmd") || strings.EqualFold(ext, ".exe") {
		manager = strings.TrimSuffix(manager, ext)
	}
	var legacy string
	if manager == "pnpm" {
		out, err := piNpmOutput(command, "list", "-g", "--depth", "0", "--json")
		if err != nil {
			return managed
		}
		var entries []struct {
			Dependencies map[string]struct{ Path string }
		}
		if err := json.Unmarshal([]byte(out), &entries); err != nil || entries == nil {
			return managed
		}
		for _, entry := range entries {
			if legacy = entry.Dependencies[name].Path; legacy != "" {
				break
			}
		}
	}
	if legacy == "" {
		args := []string{"root", "-g"}
		if manager == "bun" {
			args = []string{"pm", "bin", "-g"}
		}
		root, err := piNpmOutput(command, args...)
		if err != nil || root == "" {
			return managed
		}
		if manager == "bun" {
			root = filepath.Join(filepath.Dir(root), "install", "global", "node_modules")
		}
		legacy = filepath.Join(root, name)
	}
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return managed
}

func piNpmOutput(command []string, args ...string) (string, error) {
	// A broken package-manager wrapper must not leave doctor waiting forever.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	argv := append(append([]string(nil), command[1:]...), args...)
	cmd := exec.CommandContext(ctx, command[0], argv...)
	cmd.WaitDelay = time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if len(out) == 0 {
		out = stderr.Bytes()
	}
	return strings.TrimSpace(string(out)), err
}
