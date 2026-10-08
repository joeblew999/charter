package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// `charter adopt` takes a repo that exists to the charter way: the parts of charter any repo can
// use, whether or not it has an API. It writes what the repo lacks and keeps what it has, so
// running it again changes nothing: charter.toml, AGENTS.md, CLAUDE.md, the docs' start page and
// rules, then what `charter docs` writes, and a mise.toml that pins the tool and includes
// tasks/repo. A mise.toml the repo has is its own: adopt prints the lines to add to it. `charter
// new` makes a new API project instead, which has all of this and more.

func init() {
	commands["adopt"] = command{"[-description <text>] [-from <checkout>]",
		"take the repo you are in to the charter way, writing only what it lacks: charter.toml, AGENTS.md, CLAUDE.md, docs/README.md, docs/rules.md, the docs site, and a mise.toml with the tool's pin and the tasks any repo takes (tasks/repo), or the lines to add to the one it has", adoptCommand}
	anywhere["adopt"] = true
}

// repoTasksFolder is the folder of charter's tasks/ with the tasks any repo takes, and repoTasks
// are those tasks (a test holds the list to the folder).
const repoTasksFolder = "repo"

var repoTasks = []string{"docs:check", "docs:lint", "docs:pages", "docs:review", "docs:setup", "issue", "issues", "release", "repo", "repo:check", "repo:ci", "upstream:status"}

// The pages every repo charter keeps has, the same in a project made by new and a repo adopt took.
const (
	agentsPage = "# For agents\n\nEverything about this repo is in [docs/](docs/README.md), the same pages developers read. Read [docs/README.md](docs/README.md), then the rules, which are binding: [docs/repo/rules.md](docs/repo/rules.md), the same in every repo charter keeps, and [docs/rules.md](docs/rules.md), this repo's own.\n"
	claudePage = "@AGENTS.md\n"

	rulesHead = `---
title: Rules
nav_order: 2
---

# Rules for working in this repo

`
	ruleDocs     = "- **`docs/` is the single source of truth.** Write things down in a page here, following [writing.md](writing.md). `mise run docs:lint` checks what a program can, `mise run docs:review` has Claude check the rest.\n"
	ruleUpstream = "- **Workarounds name their upstream issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)` in the code; `mise run upstream:status` lists them.\n"

	// The rules every repo shares. A project's (goRules) add those of an API on Workers.
	// A repo's own rules: the rules every repo shares are charter's page, repo/rules.md, the same
	// everywhere, so this one starts with the pointer and is the repo's to add to.
	repoRules = rulesHead +
		"The rules every repo shares are in [Rules for every repo](repo/rules.md), which charter writes: the same page in each. Add here what is this repo's own.\n\n" +
		"- **What is generated here:** [the list](README.md#what-is-generated).\n"
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
| [repo.md](repo.md) | How every repo charter keeps is kept: [the rules they share](repo/rules.md) |
| [rules.md](rules.md) | This repo's own rules |
| [writing.md](writing.md) | The rules a page in ` + "`docs/`" + ` is held to |

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
| The tasks | ` + "`mise.toml`" + ` | [Tasks](tasks.md) lists them, written from the tasks themselves; those any repo takes come from [charter](` + anyRepoGuide + `) |

## What is generated

Never edit these: change the source and run the task.

| Path | Written by |
|---|---|
| ` + "`docs/writing.md`" + `, and the docs site's ` + "`_config.yml`, `llms.txt` and `_sass/`" + ` | ` + "`mise run docs:setup`" + ` |
| A page that ` + "`_generated.toml`" + ` in ` + "`docs/`" + ` lists | ` + "`mise run docs:setup`" + `, from its command |
| The issue forms and ` + "`labels.tsv`" + ` in ` + "`.github/`, `renovate.json`" + ` | ` + "`mise run repo`" + ` |
`
}

const anyRepoGuide = docsURL + "guides/any-repo.html"

func adoptCommand(args []string) error {
	var description, from string
	flags("adopt", args, func(f *flag.FlagSet) {
		f.StringVar(&description, "description", "", "what the repo is, in a sentence, for a new charter.toml (default: its description on GitHub, or its folder's name)")
		f.StringVar(&from, "from", "", "a checkout of charter to include tasks/repo from, with the charter on your path as the tool (default: this tool's release, pinned)")
	})
	root, err := output(started, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return errors.New("adopt runs in a git repo (git init first)")
	}
	version := toolVersion()
	switch {
	case from != "":
		if !filepath.IsAbs(from) {
			from = filepath.Join(started, from)
		}
	case version == "":
		return errors.New("adopt: run a release (mise x " + toolRelease + " -- charter adopt), or pass -from <checkout of charter>")
	}
	return adopt(filepath.FromSlash(root), description, version, from)
}

// adopt writes into the repo at root what it lacks of the charter way. version is the release the
// tool is, which mise.toml pins and includes the tasks at; with checkout (a checkout of charter)
// the tasks come from there instead and nothing is pinned.
func adopt(root, description, version, checkout string) error {
	if checkout != "" && !exists(filepath.Join(checkout, "tasks", repoTasksFolder)) {
		return fmt.Errorf("%s is not a checkout of charter with tasks/%s", checkout, repoTasksFolder)
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

	// What a repo writes itself from here on: only if it is not there.
	docsOwn := true
	if text, err := os.ReadFile(filepath.Join(root, "charter.toml")); err == nil {
		config, err := parseCharterToml(string(text))
		if err != nil {
			return err
		}
		docsOwn = config.Docs == "docs"
	}
	for _, page := range startPages(name, description) {
		path := filepath.Join(root, filepath.FromSlash(page[0]))
		switch {
		case exists(path):
			say("kept", page[0], "")
		case strings.HasPrefix(page[0], "docs/") && !docsOwn:
			say("skipped", page[0], `: charter.toml says docs = ".", so the pages are at the root and yours`)
		default:
			if err := write(path, page[1]); err != nil {
				return err
			}
			say("wrote", page[0], "")
		}
	}

	// The pointer files: charter's, the same in every repo, written again if they differ.
	pointers, err := syncFiles(root, agentFiles(), false)
	if err != nil {
		return err
	}
	for _, file := range pointers {
		say("wrote", file, "")
	}
	if len(pointers) == 0 {
		say("ok", "AGENTS.md, CLAUDE.md", ": what charter writes")
	}

	// What charter docs writes: charter's own files, the same in every repo, so written again if stale.
	switch {
	case !docsOwn:
	case onGitHub:
		files, _, err := siteFiles(root, "docs", repo)
		if err != nil {
			return err
		}
		stale, err := syncFiles(root, files, false)
		if err != nil {
			return err
		}
		for _, file := range stale {
			say("wrote", file, "")
		}
		if len(stale) == 0 {
			say("ok", "docs/", ": the docs site's files are what charter docs writes")
		}
	default:
		stale, err := syncFiles(root, map[string][]byte{"docs/writing.md": []byte(docsWriting), "docs/repo.md": []byte(docsRepo), "docs/repo/rules.md": []byte(docsRepoRules)}, false)
		if err != nil {
			return err
		}
		for _, file := range stale {
			say("wrote", file, "")
		}
		say("waiting", "docs/_config.yml", ": the docs site's config needs the repo's name on GitHub; once it is there (gh repo create), mise run docs:setup")
	}

	// mise.toml: written if there is none; one the repo has is its own, and gets the lines to add.
	pin, include := toolPin(version), tasksInclude(repoTasksFolder, version, checkout)
	file := filepath.Join(root, "mise.toml")
	have, err := os.ReadFile(file)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := write(file, startedBy("adopt", "mise.toml", adoptedTasks(name, pin, include, checkout != ""))); err != nil {
			return err
		}
		say("wrote", "mise.toml", "")
	case err != nil:
		return err
	default:
		text := string(have)
		if add := linesToAdd(text, pin, include, checkout != ""); len(add) == 0 {
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
	// The pages a command writes (docs/_generated.toml: the tasks page), now that mise.toml is there.
	if docsOwn {
		generated, err := readGenerated(filepath.Join(root, "docs"))
		if err != nil {
			return err
		}
		pages := map[string][]byte{}
		if err := addGenerated(pages, root, "docs", generated); err != nil {
			// mise cannot list the tasks yet (the tool is not installed, or the release is not out):
			// the first mise run docs:setup or mise run repo writes the page.
			say("waiting", "docs/tasks.md", ": mise could not list the tasks yet; after mise install, mise run docs:setup writes it")
		} else {
			stale, err := syncFiles(root, pages, false)
			if err != nil {
				return err
			}
			for _, file := range stale {
				say("wrote", file, "")
			}
		}
	}
	fmt.Printf(`
  mise install && mise tasks         # the tool, then what there is to run
  mise run docs:lint                 # check docs/
  mise run repo                      # with the repo on GitHub: its description and topics (charter.toml), the docs site, the issue forms, the labels, Pages

What each part is, and what it needs: %s
`, anyRepoGuide)
	return nil
}

// agentFiles are the two pointer files of a repo that is not a project: the same in every repo
// charter keeps, and charter's to write again, so what agents are told first cannot drift from repo
// to repo. What is a repo's own is in its docs/, where they point.
func agentFiles() map[string][]byte {
	mark := "<!-- Written by `charter repo` (" + charterRepo + "): don't edit. The same in every repo charter keeps; what is this repo's own is in docs/. -->\n\n"
	return map[string][]byte{
		"AGENTS.md": []byte(mark + repoAgentsPage),
		"CLAUDE.md": []byte(mark + claudePage),
	}
}

const repoAgentsPage = `# For agents

Everything about this repo is in [docs/](docs/README.md), the same pages developers read. Nothing is kept here, so there is one source of truth.

Read, in this order:

1. [docs/README.md](docs/README.md): what this repo is, and the index of every page.
2. [docs/repo/rules.md](docs/repo/rules.md): the rules every repo shares. They are binding.
3. [docs/rules.md](docs/rules.md): this repo's own rules, which add to those.
4. The open issues, ` + "`mise run issues`" + `: what is reported and what is planned. One labelled ` + "`needs-triage`" + ` comes first.
5. The page for the part you are changing, from the index.
6. [docs/writing.md](docs/writing.md) before you write or change a page in ` + "`docs/`" + `.

` + "`mise tasks`" + ` lists what there is to run; ` + "`mise run check`" + `, where the repo has it, is what must pass.

When you learn or change something, write it in the page in ` + "`docs/`" + ` it belongs to, and run ` + "`mise run docs:check`" + `. Don't add README files elsewhere.
`

// startPages are the files a repo that is not a project starts with and then owns, each with its
// "Started by" mark: one list, written by adopt on the first run and put back by repo if one goes
// missing, so there is one place that says what a repo of the charter way has.
func startPages(name, description string) [][2]string {
	pages := [][2]string{
		{"charter.toml", charterTomlFor(description)},
		{"docs/README.md", repoDocs(name)},
		{"docs/rules.md", repoRules},
		// Every repo has mise tasks, so every repo has the page of them: written by charter from
		// the tasks themselves (charter docs-tasks), fresh or the check fails.
		{"docs/_generated.toml", "# Pages a command writes: mise run docs:setup runs each and writes the page; docs:check fails on a stale one.\n[[generated]]\npage = \"tasks.md\"\nrun = \"charter docs-tasks\"\n"},
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
		add = append(add, "  under [tools]:\n    "+strings.TrimSpace(pin[strings.Index(pin, "\n")+1:]))
	}
	if !strings.Contains(text, "/tasks/"+repoTasksFolder+`"`) && !strings.Contains(text, "/tasks/"+repoTasksFolder+"?") {
		add = append(add, "  in includes, under [task_config] (a list: includes = [...]):\n    \""+include+`"`)
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
	return text + "[task_config]\n# Charter's tasks for any repo (the docs site, the repo on GitHub, the issues, the upstream issues):\n# " +
		anyRepoGuide + "\n# A task defined in this file takes the place of the included one with its name.\nincludes = [\"" + include + "\"]\n"
}
