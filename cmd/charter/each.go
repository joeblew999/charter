package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// charter each: run a mise task in every project below this folder that has it (a repo that holds several projects, as this tool's own does in examples/)
// Run by: mise run compare (mise.toml); mise run dist (mise.toml); mise run doctor (mise.toml); mise run projects:check (mise.toml); mise run release:publish (mise.toml); mise run sdk:clean (mise.toml); mise run setup (mise.toml)
func init() {
	commands["each"] = command{"[-only <name,...>] <task> [args]", "run a mise task in every project below this folder that has it (a repo that holds several projects, as this tool's own does in examples/)", each}
	anywhere["each"] = true
}

// settingsOf are the environment variables a mise.toml sets under [env].
func settingsOf(file string) map[string]bool {
	settings := map[string]bool{}
	var config struct {
		Env map[string]any `toml:"env"`
	}
	if _, err := toml.DecodeFile(file, &config); err != nil {
		return settings
	}
	for name := range config.Env {
		if name != "" && name[0] >= 'A' && name[0] <= 'Z' {
			settings[name] = true
		}
	}
	return settings
}

// each runs one mise task in every project below the current folder that defines it, one after the
// other, and stops at the first that fails. It is how a repo with several projects keeps its own
// tasks to one line: `charter each check`.
func each(args []string) error {
	var only string
	rest := flags("each", args, func(f *flag.FlagSet) {
		f.StringVar(&only, "only", "", "only these projects, by folder name, comma-separated (default: every one that has the task)")
	})
	if len(rest) == 0 {
		return errors.New("each needs <task>: a mise task that the projects below this folder define")
	}
	projects, err := projectsBelow(".")
	if err != nil {
		return err
	}
	if only != "" {
		// Those named, in the order named.
		var named []string
		for _, name := range strings.Split(only, ",") {
			at := slices.IndexFunc(projects, func(dir string) bool { return filepath.Base(dir) == name })
			if at < 0 {
				return fmt.Errorf("no project %s below this folder", name)
			}
			named = append(named, projects[at])
		}
		projects = named
	}
	ran := 0
	for _, dir := range projects {
		if !tasksOf(filepath.Join(dir, "mise.toml"))[rest[0]] {
			continue
		}
		fmt.Printf("== %s: mise run %s\n", dir, strings.Join(rest, " "))
		// Asking for a project's task is trusting its mise.toml: say so to mise, which otherwise
		// stops at each project's file the first time, in a fresh checkout or on CI.
		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		trusted := abs
		if already := os.Getenv("MISE_TRUSTED_CONFIG_PATHS"); already != "" {
			trusted = already + string(os.PathListSeparator) + abs
		}
		cmd := exec.Command("mise", append([]string{"run"}, rest...)...)
		cmd.Dir, cmd.Stdout, cmd.Stderr, cmd.Stdin = dir, os.Stdout, os.Stderr, os.Stdin
		// A project's settings are its own: what its mise.toml sets under [env] (API_URL, API_PORT)
		// is not taken from the environment this was started in, which may be another project's.
		own := settingsOf(filepath.Join(dir, "mise.toml"))
		for _, variable := range os.Environ() {
			if name, _, _ := strings.Cut(variable, "="); !own[name] {
				cmd.Env = append(cmd.Env, variable)
			}
		}
		cmd.Env = append(cmd.Env, "MISE_TRUSTED_CONFIG_PATHS="+trusted)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: mise run %s failed", dir, rest[0])
		}
		ran++
	}
	if ran == 0 {
		return fmt.Errorf("no project below this folder has the task %s", rest[0])
	}
	return nil
}
