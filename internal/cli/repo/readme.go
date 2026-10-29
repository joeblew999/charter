package repo

import (
	"fmt"
	"strings"

	"github.com/joeblew999/charter/project"
)

func cell(text string) string { return strings.NewReplacer("|", `\|`, "\n", " ").Replace(text) }

// whatToDo is docs/charter/README.md: what to do, and when, written from charter's commands in the order a developer
// meets them; then a project's steps, where the repo has projects.
func whatToDo(root string, s *project.Project) string {
	row := func(c Command) string {
		return fmt.Sprintf("| %s | `charter %s` | %s |", cell(c.When), c.Name, cell(c.About))
	}
	var repoRows, projectRows []string
	for _, c := range Commands {
		if c.When == "" {
			continue
		}
		if c.Project {
			projectRows = append(projectRows, row(c))
		} else {
			repoRows = append(repoRows, row(c))
		}
	}
	projects := projectsOf(root, project.Job{})
	var b strings.Builder
	b.WriteString(`---
title: What to do
nav_order: 100
has_children: true
---

# What to do, and when

Written by ` + "`charter init`" + ` from charter's commands: don't edit, it is the same in every repo charter keeps (after another release of charter, ` + "`charter init`" + ` writes it again).

Each is ` + "`charter <command>`" + `: the charter the repo's ` + "`mise.toml`" + ` pins (` + "`mise use github:joeblew999/charter`" + `), or ` + "`mise run charter:<command>`" + `. What it takes follows it (` + "`charter issues plan`" + `). [The rules](rules.md) are binding; [Writing docs](writing.md) says how a page is written; this repo's own pages are in [self/](../self/README.md).

| When | Run | What it does |
|---|---|---|
`)
	b.WriteString(strings.Join(repoRows, "\n") + "\n")
	switch {
	case isWorker(s):
		b.WriteString("\nThis repo is a project, whose `charter.toml` says what its steps need:\n\n| When | Run | What it does |\n|---|---|---|\n" + strings.Join(projectRows, "\n") + "\n")
	case len(projects) > 0:
		var named []string
		for _, p := range projects {
			named = append(named, "`"+p+"`")
		}
		b.WriteString("\nIn a project (" + strings.Join(named, ", ") + "), from its folder, where its `charter.toml` says what its steps need:\n\n| When | Run | What it does |\n|---|---|---|\n" + strings.Join(projectRows, "\n") + "\n")
	}
	if IsCharter(root) {
		b.WriteString("\nIn charter's own repo, charter is this checkout's: `go tool charter <command>`. Its release: `go tool charter release X.Y.Z`.\n")
	}
	b.WriteString("\nEvery command, with what it takes: `charter help`.\n")
	return b.String()
}
