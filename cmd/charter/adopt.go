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

var repoTasks = []string{"docs:check", "docs:lint", "docs:pages", "docs:review", "docs:setup", "issues", "repo", "repo:check", "upstream:status"}

// The pages every repo charter keeps has, the same in a project made by new and a repo adopt took.
const (
	agentsPage = "# For agents\n\nEverything about this repo is in [docs/](docs/README.md), the same pages developers read. Read [docs/README.md](docs/README.md), then [docs/rules.md](docs/rules.md): the rules are binding.\n"
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
	repoRules = rulesHead + ruleDocs +
		"- **Never edit what is generated** ([the list](README.md#what-is-generated)): change its source and run its task.\n" +
		ruleUpstream +
		"- **Plans are issues,** never pages in `docs/`: `charter issue plan` prints the form to fill in, `mise run issues` lists the open ones.\n" +
		"- **Only verified results go into the docs:** what ran, where, when.\n"
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
| [rules.md](rules.md) | The working rules |
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
| The tasks | ` + "`mise.toml`" + ` | ` + "`mise tasks`" + ` lists them; those any repo takes come from [charter](` + anyRepoGuide + `) |

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
	pages := [][2]string{
		{"charter.toml", charterTomlFor(description)},
		{"AGENTS.md", agentsPage},
		{"CLAUDE.md", claudePage},
		{"docs/README.md", repoDocs(name)},
		{"docs/rules.md", repoRules},
	}
	for _, page := range pages {
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
		stale, err := syncFiles(root, map[string][]byte{"docs/writing.md": []byte(docsWriting)}, false)
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
		if err := write(file, adoptedTasks(name, pin, include, checkout != "")); err != nil {
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
	fmt.Printf(`
  mise install && mise tasks         # the tool, then what there is to run
  mise run docs:lint                 # check docs/
  mise run repo                      # with the repo on GitHub: its description and topics (charter.toml), the docs site, the issue forms, the labels, Pages

What each part is, and what it needs: %s
`, anyRepoGuide)
	return nil
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
