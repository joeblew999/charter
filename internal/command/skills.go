package command

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
)

// The agent skills of everything a charter app is built with come as Claude Code plugins, the way their owners ship
// them: charter's repository is a plugin and a marketplace (.claude-plugin) that lists charter's own skill and those
// of gsx, gsxui and structpages, each from its owner's repository, where it is kept up to date. An app says so in
// its .claude/settings.json, which is committed: the marketplace, with auto-update, and the four plugins enabled.

// marketplace is charter's, and plugins what it lists.
const marketplace = "charter"

var plugins = []string{"charter", "gsx", "gsxui", "structpages"}

// agentPlugins makes sure the app's .claude/settings.json names the marketplace and enables its plugins (an app made
// with charter create has it already), and, on a machine with Claude Code, installs them for this checkout: the one
// step Claude Code leaves to each developer. charter install runs it.
func agentPlugins(p *project.Project) error {
	path := filepath.Join(p.Root, ".claude", "settings.json")
	settings := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &settings); err != nil {
			return err
		}
	}
	changed := false
	known, _ := settings["extraKnownMarketplaces"].(map[string]any)
	if known == nil {
		known = map[string]any{}
	}
	if known[marketplace] == nil {
		known[marketplace] = map[string]any{"source": map[string]any{"source": "github", "repo": "joeblew999/charter"}, "autoUpdate": true}
		settings["extraKnownMarketplaces"], changed = known, true
	}
	enabled, _ := settings["enabledPlugins"].(map[string]any)
	if enabled == nil {
		enabled = map[string]any{}
	}
	for _, name := range plugins {
		if _, ok := enabled[name+"@"+marketplace]; !ok {
			enabled[name+"@"+marketplace] = true
			settings["enabledPlugins"], changed = enabled, true
		}
	}
	if changed {
		b, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			return err
		}
		say(".claude/settings.json: charter's marketplace, and its plugins enabled (charter, gsx, gsxui, structpages)")
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		return nil // no Claude Code here: the settings are for whoever opens the app with it
	}
	have, _ := run.Output(p.Root, nil, claude, "plugin", "list")
	if _, err := run.Output(p.Root, nil, claude, "plugin", "marketplace", "add", "joeblew999/charter"); err != nil && !strings.Contains(err.Error(), "already") {
		say("· Claude Code's plugins were not installed: %v", err)
		return nil
	}
	for _, name := range plugins {
		if strings.Contains(have, name+"@"+marketplace) {
			continue
		}
		if err := run.Run(p.Root, nil, claude, "plugin", "install", name+"@"+marketplace, "--scope", "project"); err != nil {
			say("· %v", err)
		}
	}
	return nil
}
