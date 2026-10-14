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

// `charter adopt` takes a repo that exists to the charter way: the parts of charter any repo can
// use, whether or not it has an API. It writes what the repo lacks and keeps what it has, so
// running it again changes nothing: charter.toml, AGENTS.md, CLAUDE.md, the docs' start page and
// rules, then what `charter docs` writes (charter's pages in docs/charter/, the docs site's files),
// and a mise.toml that pins the tool and includes tasks/repo. A file of charter's that does not carry
// its mark is the repo's own, and kept. A mise.toml the repo has is its own: adopt prints the lines
// to add to it. In a project (charter's own repo among them) AGENTS.md, CLAUDE.md and the start pages
// are the project's: adopt leaves them. `charter new` makes a new API project instead.

func init() {
	commands["adopt"] = command{"[-description <text>] [-from <checkout>] [-check]",
		"write charter's kit into the repo you are in, charter's own included: docs/charter/, the docs site's files, the issue forms, the workflows, AGENTS.md, CLAUDE.md, renovate.json, never a file of the repo's own; and what it lacks of charter.toml, docs/README.md, docs/rules.md and the mise.toml lines. Run again, it brings the kit to this release; -check writes nothing and fails if the kit differs", adoptCommand}
	anywhere["adopt"] = true
}

// repoTasksFolder is the folder of charter's tasks/ with the tasks any repo takes, and repoTasks
// are those tasks (a test holds the list to the folder).
const repoTasksFolder = "repo"

var repoTasks = []string{"adopt", "docs:check", "docs:lint", "docs:pages", "docs:review", "docs:setup", "issue", "issues", "release", "repo", "repo:check", "repo:ci", "upstream:status"}

// The pages every repo charter keeps has, the same in a project made by new and a repo adopt took.
const (
	agentsPage = "# For agents\n\nEverything about this repo is in [docs/](docs/README.md), the same pages developers read. The start page, the rules and how to use charter come with this file: each line below that starts with @ is that page itself, for an agent that reads them (Claude does). Any other agent: read [docs/README.md](docs/README.md) first, then the rules, which are binding: [docs/charter/rules.md](docs/charter/rules.md), the same in every repo charter keeps, and [docs/rules.md](docs/rules.md), this repo's own; then [docs/charter/using.md](docs/charter/using.md), the tool it is kept with. Charter's part of the repo is [docs/charter/](docs/charter/README.md).\n\n@docs/README.md\n@docs/charter/rules.md\n@docs/rules.md\n@docs/charter/using.md\n"
	claudePage = "@AGENTS.md\n"

	rulesHead = `---
title: Rules
nav_order: 2
---

# Rules for working in this repo

`
	ruleDocs     = "- **`docs/` is the single source of truth.** Write things down in a page here, following [Writing docs](charter/writing.md). `mise run docs:lint` checks what a program can, `mise run docs:review` has Claude check the rest.\n"
	ruleUpstream = "- **Workarounds name their upstream issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)` in the code; `mise run upstream:status` lists them.\n"

	// The rules every repo shares. A project's (goRules) add those of an API on Workers.
	// A repo's own rules: the rules every repo shares are charter's page, charter/rules.md, the same
	// everywhere, so this one starts with the pointer and is the repo's to add to.
	repoRules = rulesHead +
		"The rules every repo shares are in [Rules for every repo](charter/rules.md), which charter writes: the same page in each. Add here what is this repo's own.\n\n" +
		"- **What charter writes is charter's:** [the list](charter/README.md#what-is-charters). Never edit it here.\n"
)

// startPage is the top of a repo's docs/README.md, the docs site's home page: its front matter, its
// name, the index of its pages, and the heading the map of the repo goes under.
func startPage(name string) string {
	return `---
title: Start here
nav_order: 1
permalink: /
---

# ` + name + `

Everything written about this repo lives in this folder. ` + "`AGENTS.md`" + ` only points here.

| Page | What it covers |
|---|---|
| This page | What is what |
| [rules.md](rules.md) | This repo's own rules |
| [charter/](charter/README.md) | Charter's part, the same in every repo it keeps: [the rules they share](charter/rules.md), [using charter](charter/using.md), [writing docs](charter/writing.md), [the tasks](charter/tasks.md) |

## What is what
`
}

// repoDocs is the docs/README.md adopt writes: the start page, with the map left to the repo and
// the list of what charter generates in it.
func repoDocs(name string) string {
	return startPage(name) + `
Say here what this repo is, and where its parts are: a row per part.

| Name | Where | What it is |
|---|---|---|
| The tasks | ` + "`mise.toml`" + ` | [Tasks](charter/tasks.md) lists them, written from the tasks themselves; those any repo takes come from [charter](` + anyRepoGuide + `) |

## What is generated

Never edit these: change the source and run the task.

| Path | Written by |
|---|---|
| ` + "`docs/charter/`" + `, and the docs site's ` + "`_config.yml`, `llms.txt` and `_sass/`" + ` | ` + "`mise run docs:setup`" + ` |
| A page that ` + "`_generated.toml`" + ` in ` + "`docs/`" + ` lists | ` + "`mise run docs:setup`" + `, from its command |
| ` + "`.github/ISSUE_TEMPLATE/charter-*.yml`, `config.yml`, `.github/workflows/charter.yml`, `renovate.json`, `AGENTS.md`, `CLAUDE.md`" + ` | ` + "`mise run repo`" + ` |
`
}

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

	// What a repo writes itself from here on: only if it is not there. A project's start pages are
	// what charter new wrote (and charter's own repo has its own): only charter.toml, if it lacks one.
	config := charterToml{Docs: "docs", Renovate: true, Workflow: true}
	if text, err := os.ReadFile(filepath.Join(root, "charter.toml")); err == nil {
		if config, err = parseCharterToml(string(text)); err != nil {
			return err
		}
	}
	docsOwn, project := config.Docs == "docs", hasProjects(root, config)
	for _, page := range startPages(name, description) {
		path := filepath.Join(root, filepath.FromSlash(page[0]))
		switch {
		case exists(path):
			say("kept", page[0], "")
		case project && page[0] != "charter.toml":
			say("skipped", page[0], ": a project's start pages are its own")
		case strings.HasPrefix(page[0], "docs/") && !docsOwn:
			say("skipped", page[0], `: charter.toml says docs = ".", so the pages are at the root and yours`)
		default:
			if err := write(path, page[1]); err != nil {
				return err
			}
			say("wrote", page[0], "")
		}
	}

	if project {
		say("kept", "AGENTS.md, CLAUDE.md", ": a project's own")
	} else {
		// What a repo's own CLAUDE.md or AGENTS.md said is its knowledge: it moves into docs/rules.md,
		// where charter keeps a repo's own rules, before charter's pointer files take their place.
		for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
			moved, err := keepAgentNotes(root, name, docsOwn)
			if err != nil {
				return err
			}
			if moved {
				say("moved", name, ": what it said is now in docs/rules.md")
			}
		}
		// The pointer files are the kit's: what the repo's own said is in docs/rules.md now, so they go
		// and the kit writes charter's in their place.
		for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
			if have, err := os.ReadFile(filepath.Join(root, name)); err == nil && docsOwn {
				if charters, _, ok := markOf(have); !ok || !charters {
					if err := os.Remove(filepath.Join(root, name)); err != nil {
						return err
					}
				}
			}
		}
	}

	// mise.toml: written if there is none; one the repo has is its own, and gets the lines to add.
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
		if add := linesToAdd(text, pin, include, checkout != "" || isCharter(root)); len(add) == 0 {
			say("kept", "mise.toml", ": it has charter's lines (tasks/"+repoTasksFolder+" included)")
		} else {
			say("kept", "mise.toml", ": it is yours. Add to it:\n"+strings.Join(add, "\n"))
		}
		var own []string
		for _, m := range taskDef.FindAllStringSubmatch(text, -1) {
			if slices.Contains(repoTasks, m[1]) {
				own = append(own, m[1])
			}
		}
		if len(own) > 0 {
			fmt.Printf("         mise.toml defines %s itself: each takes the place of charter's task of that name. Delete those you want from charter\n", strings.Join(own, ", "))
		}
	}
	// The kit, now that mise.toml is there (the tasks page is written from it): every file charter
	// writes, as this release has it.
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
	fmt.Printf(`
  mise install && mise tasks         # the tool, then what there is to run
  mise run docs:lint                 # check docs/
  mise run repo                      # with the repo on GitHub: its description and topics (charter.toml), the docs site, the issue forms, the labels, Pages

What each part is, and what it needs: %s
`, anyRepoGuide)
	return nil
}

// keepAgentNotes moves what a repo's own agent file says (one without charter's mark, and more than
// a pointer) to the end of docs/rules.md: headings one level down, links made relative to docs/.
// True when it moved something.
func keepAgentNotes(root, name string, docsOwn bool) (bool, error) {
	text, err := os.ReadFile(filepath.Join(root, name))
	if err != nil || strings.Contains(string(text), "Written by `charter") {
		return false, nil
	}
	notes := strings.TrimSpace(string(text))
	if notes == "" || notes == "@AGENTS.md" || !docsOwn {
		return false, nil
	}
	notes = ownHeading.ReplaceAllString(notes, "#$1")
	notes = relativeLink.ReplaceAllString(notes, "](../$1)")
	rules := filepath.Join(root, "docs", "rules.md")
	have, _ := os.ReadFile(rules)
	section := "\n## What " + name + " said before charter adopt\n\nMoved here by `charter adopt`, as it was. Yours to edit: keep what is still true, where it belongs.\n\n" + notes + "\n"
	return true, write(rules, strings.TrimRight(string(have), "\n")+"\n"+section)
}

var (
	ownHeading   = regexp.MustCompile(`(?m)^(#{1,5} )`)
	relativeLink = regexp.MustCompile(`\]\(((?:[^)#:/][^):]*|\./[^):]*))\)`)
)

// agentFiles are the two pointer files of a repo that is not a project, from the kit: the same in
// every repo charter keeps. What is a repo's own is in its docs/, where they point.
func agentFiles() map[string][]byte {
	return map[string][]byte{"AGENTS.md": []byte(kitFile("AGENTS.md")), "CLAUDE.md": []byte(kitFile("CLAUDE.md"))}
}

// startPages are the files a repo that is not a project starts with and then owns, each with its
// "Started by" mark: one list, written by adopt on the first run and put back by repo if one goes
// missing, so there is one place that says what a repo of the charter way has.
func startPages(name, description string) [][2]string {
	pages := [][2]string{
		{"charter.toml", charterTomlFor(description)},
		{"docs/README.md", repoDocs(name)},
		{"docs/rules.md", repoRules},
	}
	for i := range pages {
		pages[i][1] = startedBy("adopt", pages[i][0], pages[i][1])
	}
	return pages
}

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
