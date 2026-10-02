package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

func init() {
	commands["upstream"] = command{"", "the upstream issues the code under this folder works around (every `Upstream:` tag) and their state", upstream}
	commands["doctor"] = command{"", "check what the project's tasks need: npm installs, Docker, Go, Rust, leftover containers", doctor}
	commands["each"] = command{"[-only <name,...>] <task> [args]", "run a mise task in every project below this folder that has it (a repo that holds several projects, as this tool's own does in examples/)", each}
	anywhere["upstream"], anywhere["each"] = true, true
}

// A tag is a comment line such as:  Upstream: owner/repo#123 (when fixed: what to do then)
var upstreamTag = regexp.MustCompile(`^([^:]+:[0-9]+):.*Upstream: ([\w.-]+/[\w.-]+)#([0-9]+) *(.*)$`)

// upstream finds the tags and asks GitHub for each issue's state. CLOSED means that workaround can
// go (the table in docs/upstream.md says what to do).
func upstream([]string) error {
	// Not where a tag is written about: the docs, the task descriptions, this file and the tool's tests.
	out, err := output(".", "git", "grep", "-n", "-E", `Upstream: [A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#[0-9]+`, "--", ":!*.md", ":!*mise.toml", ":!cmd/charter/misc.go", ":!cmd/charter/*_test.go")
	if err != nil {
		return errors.New("no Upstream: tags found")
	}
	places := map[string][]string{}
	for _, line := range strings.Split(out, "\n") {
		if m := upstreamTag.FindStringSubmatch(line); m != nil {
			issue := m[2] + "#" + m[3]
			places[issue] = append(places[issue], fmt.Sprintf("        %s  %s", m[1], m[4]))
		}
	}
	issues := make([]string, 0, len(places))
	for issue := range places {
		issues = append(issues, issue)
	}
	sort.Strings(issues)
	closed := 0
	for _, issue := range issues {
		repo, number, _ := strings.Cut(issue, "#")
		state, title := "UNREACHABLE", ""
		if info, err := output(".", "gh", "api", "repos/"+repo+"/issues/"+number, "--jq", `.state + "\t" + .title`); err == nil {
			state, title, _ = strings.Cut(info, "\t")
			state = strings.ToUpper(state)
		}
		if state == "CLOSED" {
			closed++
		}
		fmt.Printf("%s  %s  %s\n%s\n", state, issue, title, strings.Join(places[issue], "\n"))
	}
	fmt.Printf("\n%d upstream issues, %d closed", len(issues), closed)
	if closed > 0 {
		fmt.Print(": remove those workarounds (docs/upstream.md)")
	}
	fmt.Println()
	return nil
}

func doctor([]string) error {
	failed := false
	ok := func(format string, a ...any) { fmt.Printf("  ok    "+format+"\n", a...) }
	warn := func(format string, a ...any) { fmt.Printf("  WARN  "+format+"\n", a...) }
	bad := func(format string, a ...any) { fmt.Printf("  FAIL  "+format+"\n", a...); failed = true }
	if exists("node_modules") {
		ok("npm packages installed")
	} else {
		bad("no npm packages: mise run setup")
	}
	if docker() == nil {
		ok("docker running (Fern generates in containers)")
	} else {
		bad("docker not running: start Docker")
	}
	for _, tool := range []struct{ name, arg, why string }{
		{"go", "version", "the tool itself, a Go Worker, sdk:check on Go SDKs"},
		{"cargo", "--version", "the Fern CLI builds"},
		{"gh", "--version", "upstream:status, docs:setup, release"},
	} {
		if out, err := exec.Command(tool.name, tool.arg).Output(); err == nil {
			ok("%s (%s)", strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0], tool.why)
		} else {
			warn("no %s: mise install (%s)", tool.name, tool.why)
		}
	}
	if os.Getenv("FERN_TOKEN") != "" {
		ok("FERN_TOKEN set")
	} else {
		warn("FERN_TOKEN not set: Fern documents local generation as an Enterprise feature needing it (it ran without one on 2026-10-01)")
	}
	if ids, _ := output(".", "docker", "ps", "-q", "--filter", wiremock); ids == "" {
		ok("no leftover WireMock containers")
	} else {
		warn("%d WireMock container(s) running: mise run sdk:clean", len(strings.Fields(ids)))
	}
	if failed {
		return errors.New("not ready")
	}
	fmt.Print("SDK groups: ")
	return sdkList(nil)
}

// projectsBelow are the projects in the folders below dir, one or two levels down (examples/<name>).
func projectsBelow(dir string) ([]string, error) {
	var found []string
	for _, pattern := range []string{"*/mise.toml", "*/*/mise.toml"} {
		files, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if isProject(filepath.Dir(file)) {
				found = append(found, filepath.Dir(file))
			}
		}
	}
	sort.Strings(found)
	return found, nil
}

// tasksOf are the tasks a mise.toml defines itself (mise also offers a folder the tasks of the
// mise.toml files above it, which is not what `each` asks).
func tasksOf(file string) map[string]bool {
	tasks := map[string]bool{}
	if content, err := os.ReadFile(file); err == nil {
		for _, m := range taskDef.FindAllStringSubmatch(string(content), -1) {
			tasks[m[1]] = true
		}
	}
	return tasks
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
		cmd.Env = append(os.Environ(), "MISE_TRUSTED_CONFIG_PATHS="+trusted)
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
