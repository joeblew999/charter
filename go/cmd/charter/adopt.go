package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// charter adopt: write charter's kit into the repo you are in, charter's own included: docs/charter/, the docs site's files, the issue forms, the workflows, AGENTS.md, CLAUDE.md, renovate.json, never a file of the repo's own; and what it lacks of charter.toml, docs/README.md, docs/rules.md and the mise.toml lines. Run again, it brings the kit to this release; -check writes nothing and fails if the kit differs
// Run by: mise run adopt (tasks/repo/tasks.toml); mise run charter:check (mise.toml); mise run docs:check (tasks/repo/tasks.toml); mise run adopt (tasks/repo/tasks.toml)
func init() {
	commands["adopt"] = command{"[-description <text>] [-from <checkout>] [-check]",
		"write charter's kit into the repo you are in, charter's own included: docs/charter/, the docs site's files, the issue forms, the workflows, AGENTS.md, CLAUDE.md, renovate.json, never a file of the repo's own; and what it lacks of charter.toml, docs/README.md, docs/rules.md and the mise.toml lines. Run again, it brings the kit to this release; -check writes nothing and fails if the kit differs", adoptCommand}
	anywhere["adopt"] = true
}

var repoTasks = []string{"adopt", "docs:check", "docs:lint", "docs:pages", "docs:review", "adopt", "issue", "issues", "release", "repo", "repo:check", "repo:ci", "upstream:status"}

const anyRepoGuide = docsURL + "guides/any-repo.html"

func adoptCommand(args []string) error {
	var description, from string
	var check bool
	flags("adopt", args, func(f *flag.FlagSet) {
		f.BoolVar(&check, "check", false, "write nothing: fail if a file of the kit differs from what this release writes")
		f.StringVar(&description, "description", "", "what the repo is, in a sentence, for a new charter.toml (default: its description on GitHub, or its folder's name)")
		f.StringVar(&from, "from", "", "a checkout of charter to include tasks/repo from, with the charter on your path as the tool (default: this tool's release, pinned)")
	})
	root, err := output(started, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return errors.New("adopt runs in a git repo (git init first)")
	}
	if check {
		return adoptCheck(filepath.FromSlash(root))
	}
	if from != "" && !filepath.IsAbs(from) {
		from = filepath.Join(started, from)
	}
	return adopt(filepath.FromSlash(root), description, toolVersion(), from)
}

// adopt writes into the repo at root what it lacks of the charter way. version is the release the
// tool is, which mise.toml pins and includes the tasks at; with checkout (a checkout of charter)
// the tasks come from there instead and nothing is pinned.
func adoptCheck(root string) error {
	changed, kept, _, err := writeKit(root, true)
	if err != nil {
		return err
	}
	for _, file := range kept {
		fmt.Printf("kept     %s: the repo's own (it does not carry charter's mark)\n", file)
	}
	for _, file := range changed {
		fmt.Fprintf(os.Stderr, "differs  %s: charter adopt writes it\n", strings.TrimPrefix(file, "removed ")+map[bool]string{true: " (not the kit's: it goes)", false: ""}[strings.HasPrefix(file, "removed ")])
	}
	if len(changed) > 0 {
		return fmt.Errorf("%d files of the kit differ from what this release writes: charter adopt", len(changed))
	}
	fmt.Println("ok       the kit: every file is what this release writes")
	return nil
}

func adopt(root, description, version, checkout string) error {
	if checkout != "" && !exists(filepath.Join(checkout, "tasks", repoTasksFolder)) {
		return fmt.Errorf("%s is not a checkout of charter with tasks/%s", checkout, repoTasksFolder)
	}
	// A mise.toml adopt writes pins the release this tool is: without one to pin, stop before writing anything.
	if !exists(filepath.Join(root, "mise.toml")) && version == "" && checkout == "" {
		return errors.New("adopt: the repo has no mise.toml, and this tool is no release to pin: run a release (mise x " + toolRelease + "@X.Y.Z -- charter adopt), or pass -from <checkout of charter>")
	}
	// The repo on GitHub, if it is there yet: its description, and what the docs site is made from.
	var repo githubRepo
	onGitHub := false
	if out, err := gh(root, "repo", "view", "--json", "nameWithOwner,name,description,defaultBranchRef"); err == nil {
		onGitHub = json.Unmarshal([]byte(out), &repo) == nil && repo.Name != ""
	}
	name := filepath.Base(root)
	if onGitHub {
		name = repo.Name
	}
	if description == "" {
		if description = repo.Description; description == "" {
			description = name
		}
	}
	say := func(did, path, why string) { fmt.Printf("%-7s  %s%s\n", did, path, why) }

	config, err := readCharterToml(root)
	if err != nil {
		return err
	}

	// 1. charter.toml, if the repo has none: its description, for GitHub.
	if path := filepath.Join(root, "charter.toml"); exists(path) {
		say("kept", "charter.toml", "")
	} else if err := write(path, startedBy("adopt", "charter.toml", charterTomlFor(description))); err != nil {
		return err
	} else {
		say("wrote", "charter.toml", "")
	}

	// 2. mise.toml: written if there is none; one the repo has is its own, and gets the lines to add.
	pin, include := toolPin(version), tasksInclude(repoTasksFolder, version, checkout)
	file := filepath.Join(root, "mise.toml")
	have, err := os.ReadFile(file)
	switch {
	case errors.Is(err, os.ErrNotExist) && version == "" && checkout == "":
		return errors.New("adopt: the repo has no mise.toml, and this tool is no release to pin: run a release (mise x " + toolRelease + "@X.Y.Z -- charter adopt), or pass -from <checkout of charter>")
	case errors.Is(err, os.ErrNotExist):
		if err := write(file, startedBy("adopt", "mise.toml", adoptedTasks(name, pin, include, checkout != ""))); err != nil {
			return err
		}
		say("wrote", "mise.toml", "")
	case err != nil:
		return err
	default:
		text := string(have)
		// No release to pin (a checkout's binary, or charter's own repo): no pin to add.
		if add := linesToAdd(text, pin, include, checkout != "" || version == "" || isCharter(root)); len(add) == 0 {
			say("kept", "mise.toml", ": it has charter's lines (tasks/"+repoTasksFolder+" included)")
		} else {
			say("kept", "mise.toml", ": it is yours. Add to it:\n"+strings.Join(add, "\n"))
		}
		var own []string
		names, _ := tomlTasks([]byte(text))
		for _, name := range names {
			if slices.Contains(repoTasks, name) {
				own = append(own, name)
			}
		}
		if len(own) > 0 {
			fmt.Printf("         mise.toml defines %s itself: each takes the place of charter's task of that name. Delete those you want from charter\n", strings.Join(own, ", "))
		}
	}
	// 3. The kit, now that mise.toml is there (the tasks page is written from it).
	if err := writeKitSaying(root, say); err != nil {
		return err
	}

	// 4. A repo's own AGENTS.md or CLAUDE.md: what it said moves into docs/rules.md, and the kit's take
	// their place. A project's are its own.
	if !hasProjects(root, config) && config.Docs == "docs" {
		took := false
		for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
			moved, err := takeAgentFile(root, name)
			if err != nil {
				return err
			}
			if moved {
				say("moved", name, ": what it said is now in docs/rules.md")
			}
			took = took || moved || !markedCharters(filepath.Join(root, name))
		}
		if took {
			if err := writeKitSaying(root, say); err != nil {
				return err
			}
		}
	}
	fmt.Print("\nNext: mise trust && mise install, then the tasks in the order docs/charter/tasks.md lists them.\n")
	return nil
}

// takeAgentFile moves what a repo's own agent file says (one without charter's mark, and more than a
// pointer) to the end of docs/rules.md, headings one level down and links made relative to docs/, and
// removes the file, so the kit's takes its place. True when it moved something.
func takeAgentFile(root, name string) (bool, error) {
	path := filepath.Join(root, name)
	text, err := os.ReadFile(path)
	if err != nil || markedCharters(path) {
		return false, nil
	}
	notes := strings.TrimSpace(string(text))
	if notes != "" && notes != "@AGENTS.md" {
		notes = ownHeading.ReplaceAllString(notes, "#$1")
		notes = relativeLink.ReplaceAllString(notes, "](../$1)")
		rules := filepath.Join(root, "docs", "rules.md")
		have, _ := os.ReadFile(rules)
		section := "\n## What " + name + " said before charter adopt\n\nMoved here by `charter adopt`, as it was. Yours to edit: keep what is still true, where it belongs.\n\n" + notes + "\n"
		if err := write(rules, strings.TrimRight(string(have), "\n")+"\n"+section); err != nil {
			return false, err
		}
	}
	return notes != "" && notes != "@AGENTS.md", os.Remove(path)
}

// writeKitSaying writes the kit into the repo at root and says what it did.
func writeKitSaying(root string, say func(did, path, why string)) error {
	changed, kept, waiting, err := writeKit(root, false)
	if err != nil {
		return err
	}
	for _, file := range changed {
		if strings.HasPrefix(file, "removed ") {
			fmt.Println(file)
		} else {
			say("wrote", file, "")
		}
	}
	for _, file := range kept {
		say("kept", file, ": yours (it does not carry charter's mark), so charter leaves it")
	}
	for file, why := range waiting {
		say("waiting", filepath.ToSlash(file), ": "+why+"; charter adopt again writes it")
	}
	if len(changed) == 0 {
		say("ok", "the kit", ": every file is what this release writes")
	}
	return nil
}

var (
	ownHeading   = regexp.MustCompile(`(?m)^(#{1,5} )`)
	relativeLink = regexp.MustCompile(`\]\(((?:[^)#:/][^):]*|\./[^):]*))\)`)
)

// linesToAdd are the lines a mise.toml the repo wrote lacks, each under where it goes: the tool's pin
// (not from a checkout) and the include of tasks/repo. None: it has both, at whatever release.
func linesToAdd(text, pin, include string, checkout bool) []string {
	var add []string
	if !checkout && !strings.Contains(text, `"`+toolRelease+`"`) {
		// The pin's own line, after its comment.
		add = append(add, "  under [tools]:\n    "+strings.TrimSpace(pin[strings.LastIndex(strings.TrimSpace(pin), "\n")+1:]))
	}
	if !strings.Contains(text, "/tasks/"+repoTasksFolder+`"`) && !strings.Contains(text, "/tasks/"+repoTasksFolder+"?") && !strings.Contains(text, `"tasks/`+repoTasksFolder+`"`) {
		add = append(add, "  in includes, under [task_config] (a list: includes = [...]):\n    \""+include+`"`)
	}
	if !checkout && !strings.Contains(text, "\nexperimental = true") {
		add = append(add, "  under [settings] (mise takes the include from GitHub only with it):\n    experimental = true")
	}
	return add
}

// adoptedTasks is the mise.toml adopt writes for a repo that has none: the tool's pin (not from a
// checkout, whose tool is the charter on the path) and the include of the tasks any repo takes.
func adoptedTasks(name, pin, include string, checkout bool) string {
	text := "# " + name + ": its tasks. `mise tasks` lists them; `mise run <task>` runs one.\n\n"
	if checkout {
		text += "# The charter tool the tasks run is the one on your path: adopt -from pins none.\n\n"
	} else {
		text += "[tools]\n" + pin + "\n"
	}
	text += "[task_config]\n# Charter's tasks for any repo (the docs site, the repo on GitHub, the issues, the upstream issues):\n# " +
		anyRepoGuide + "\n# A task defined in this file takes the place of the included one with its name.\nincludes = [\"" + include + "\"]\n"
	if checkout {
		return text
	}
	return withGitIncludes(text)
}

// hasProjects reports whether the repo at root is a project or lists some.
func hasProjects(root string, config charterToml) bool {
	return len(config.Projects) > 0 || isProject(root)
}
