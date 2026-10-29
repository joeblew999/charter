package repo

import (
	"embed"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joeblew999/charter/internal/pins"
	"github.com/joeblew999/charter/internal/version"
	"github.com/joeblew999/charter/project"
)

// The files charter keeps the same in every repo: its pages in docs/charter/, which say what to do and when; the docs
// site's files GitHub Pages reads at the top of docs/; the issue forms, the labels and the job runner in .github/;
// AGENTS.md, CLAUDE.md, renovate.json. They are changed here, and only here.
//
//go:embed all:files
var files embed.FS

// folder is charter's folder in a repo's docs.
const folder = "charter"

// mark matches the lines a file charter writes starts with: Written by `charter <command>` … don't edit (charter's),
// or Started by `charter <command>` … yours to edit (the repo's, written once). charter-ts's marks are charter's too.
var mark = regexp.MustCompile("(Written|Started) by `charter(?:-ts)? ([a-z-]+)`")

// marked is a file's mark: whether charter wrote it, and whether it only started it.
func marked(content string) (charters, started bool) {
	lines := strings.Split(content, "\n")
	head := strings.Join(lines[:min(len(lines), 12)], "\n")
	m := mark.FindStringSubmatch(head)
	if m == nil {
		return false, false
	}
	if m[1] == "Written" && strings.Contains(head, "don't edit") {
		return true, false
	}
	return false, m[1] == "Started" && strings.Contains(head, "yours to edit")
}

const startMark = "Started by `charter init` (github.com/joeblew999/charter): yours to edit."

// isWorker is whether the repo's top is a Worker's (charter.toml names one), a project in a repo of its own.
func isWorker(s *project.Project) bool { return s.IsWorker() }

// starters are what charter init starts a repo with where it has none, then the repo's own: the tools it needs
// (mise.toml), its line ends and generated files (.gitattributes), and in docs/self/, the page that says what it is and
// its own rules.
func starters(root string, s *project.Project) map[string]string {
	has := func(name string) bool { _, err := os.Stat(filepath.Join(root, name)); return err == nil }
	var tools []string
	if released := version.String(); version.Released() {
		tools = append(tools, fmt.Sprintf("%q = %q", "github:joeblew999/charter", strings.TrimPrefix(released, "v")))
	}
	repoPins := pins.In(pins.Repo)
	pin := func(name string) {
		key := name
		if strings.ContainsAny(name, ":/") {
			key = fmt.Sprintf("%q", name)
		}
		tools = append(tools, fmt.Sprintf("%s = %q", key, repoPins[name]))
	}
	if has("package.json") {
		pin("node")
	}
	if isWorker(s) {
		pin("github:jdx/fnox")
		pin("workerd")
	}
	if has("go.mod") {
		pin("go")
		pin("aqua:sqlc-dev/sqlc")
	}
	out := map[string]string{
		"mise.toml": "# " + startMark + "\n# The tools, which mise installs here and on GitHub (charter-ci.yml): charter, and what it runs. TinyGo and\n# wasm-opt are charter's (.config/mise/conf.d/charter.toml, in a Go app).\n[tools]\n" + strings.Join(tools, "\n") + "\n",
		".gitattributes": "# " + startMark + `
# Line ends are LF on every system: the checks compare generated files with the committed ones byte
# for byte, and gofmt wants LF. (Git on Windows would otherwise check files out with CRLF.)
* text=auto eol=lf

# Generated files: nobody edits these by hand. GitHub marks them "generated" and folds them in diffs.
fern/openapi.json                   linguist-generated=true
fern/asyncapi.json                  linguist-generated=true
sdk/go/**                           linguist-generated=true
*.x.go                              linguist-generated=true
package-lock.json                   linguist-generated=true
.github/workflows/self-*.yml        linguist-generated=true
.github/workflows/charter-repo.yml  linguist-generated=true
docs/charter/README.md              linguist-generated=true
`,
		"docs/self/README.md": `---
# ` + startMark + `
title: Start here
nav_order: 1
permalink: /
---

# Start here

This repo's own pages; ` + "`AGENTS.md`" + ` only points here. Say here what this repo is, and where its parts are. What to do, and when: [docs/charter/](../charter/README.md).

| Page | What it is |
|---|---|
| [rules.md](rules.md) | This repo's own rules |
`,
		"docs/self/rules.md": `---
# ` + startMark + `
title: Rules
nav_order: 2
---

# Rules for working in this repo

The rules every repo shares are in [Rules for every repo](../charter/rules.md). Add here what is this repo's own.
`,
	}
	return out
}

// want is every file charter writes into the repo at root, by path in the repo: charter's own, less what the repo
// does not take ([github] renovate = false: renovate.json; workflows = false: the workflows); the starters;
// docs/charter/README.md; and the workflows of charter's job and the repo's own ([ci.jobs]).
func want(root string, s *project.Project) (map[string]string, error) {
	docs := s.GitHub.DocsFolder()
	inDocs := func(rel string) string {
		if after, ok := strings.CutPrefix(rel, "docs/"); ok {
			return path.Join(docs, after)
		}
		return rel
	}
	workflows := s.GitHub.Workflows == nil || *s.GitHub.Workflows
	out := map[string]string{}
	err := fs.WalkDir(files, "files", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, "files/")
		if rel == "renovate.json" && s.GitHub.Renovate != nil && !*s.GitHub.Renovate {
			return nil
		}
		if strings.HasPrefix(rel, ".github/workflows/") && !workflows {
			return nil
		}
		b, err := files.ReadFile(p)
		out[inDocs(rel)] = string(b)
		return err
	})
	if err != nil {
		return nil, err
	}
	for rel, content := range starters(root, s) {
		out[inDocs(rel)] = content
	}
	out[path.Join(docs, folder, "README.md")] = whatToDo(root, s)
	if workflows {
		jobs, err := Jobs(root)
		if err != nil {
			return nil, err
		}
		for name, j := range jobs {
			out[".github/workflows/"+workflowOf(name)] = workflow(root, name, j)
		}
	}
	return out, nil
}

// ownPlace is a path in one of charter's own places, which are charter's whatever is in them: its folder in the
// docs, its forms and workflows (charter-*). Other names are the repo's.
func ownPlace(name string) bool {
	return strings.HasPrefix(name, folder+"/") || strings.Contains(name, "/"+folder+"/") || strings.HasPrefix(path.Base(name), "charter-")
}

// Write writes charter's files into the repo at root: charter's own places as this charter has them, with anything
// else in them removed; a started file only where there is none; and a file at a name the repo may use only where
// it is charter's. With checkOnly it writes nothing. It returns what it wrote (or would), and the repo's own files it
// kept.
func Write(root string, checkOnly bool) (changed, kept []string, err error) {
	s, err := settings(root)
	if err != nil {
		return nil, nil, err
	}
	all, err := want(root, s)
	if err != nil {
		return nil, nil, err
	}
	for _, name := range slices.Sorted(maps.Keys(all)) {
		content := all[name]
		p := filepath.Join(root, filepath.FromSlash(name))
		_, started := marked(content)
		have, err := os.ReadFile(p)
		if err == nil {
			if started || string(have) == content {
				continue
			}
			if charters, _ := marked(string(have)); !ownPlace(name) && !charters {
				kept = append(kept, name)
				continue
			}
		}
		changed = append(changed, name)
		if !checkOnly {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return nil, nil, err
			}
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				return nil, nil, err
			}
		}
	}
	// What is in charter's places and no longer charter's goes: a page, form or workflow of an older charter, and a
	// workflow of a job that is gone.
	docs := s.GitHub.DocsFolder()
	for _, place := range []struct {
		dir   string
		match func(string, string) bool
	}{
		{path.Join(docs, folder), func(string, string) bool { return true }},
		{".github/ISSUE_TEMPLATE", func(name, _ string) bool { return strings.HasPrefix(name, "charter-") }},
		{".github/workflows", func(name, content string) bool {
			charters, _ := marked(content)
			return strings.HasPrefix(name, "charter-") || strings.HasPrefix(name, "self-") && charters
		}},
	} {
		dir := filepath.Join(root, filepath.FromSlash(place.dir))
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			name := filepath.ToSlash(rel)
			b, _ := os.ReadFile(p)
			if _, ok := all[name]; ok || !place.match(path.Base(name), string(b)) {
				return nil
			}
			changed = append(changed, "removed "+name)
			if !checkOnly {
				return os.Remove(p)
			}
			return nil
		})
	}
	return changed, kept, nil
}

// Problems are what differs between the repo at root and what this charter writes into it, as charter check and
// charter init -check say them; and a package.json script that runs charter-ts, which is gone.
func Problems(root string) ([]string, error) {
	changed, _, err := Write(root, true)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, name := range changed {
		if gone, ok := strings.CutPrefix(name, "removed "); ok {
			out = append(out, gone+": not charter's any more (charter init removes it)")
		} else {
			out = append(out, name+": not what this charter writes (charter init)")
		}
	}
	if b, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		for _, m := range regexp.MustCompile(`"([a-z:-]+)":\s*"charter-ts ([a-z:-]+)[^"]*"`).FindAllStringSubmatch(string(b), -1) {
			out = append(out, fmt.Sprintf("package.json: the script %s runs charter-ts, which is gone: delete it (charter %s, or mise run charter:%s)", m[1], m[2], m[2]))
		}
	}
	return out, nil
}

// Init writes charter's files into the repo at root, saying what it did: what it wrote, what it removed, and the
// repo's own files at charter's names it left alone. A repo's own CLAUDE.md or AGENTS.md (one charter did not write)
// first moves into docs/self/rules.md, so charter's take their place; a project's are its own.
func Init(root string) error {
	s, err := settings(root)
	if err != nil {
		return err
	}
	// A repo's own agent files are taken after charter's files are written (docs/self/rules.md among them), so they
	// are not reported kept.
	taking := !isWorker(s) && s.GitHub.DocsFolder() == "docs"
	quiet := map[bool][]string{true: {"CLAUDE.md", "AGENTS.md"}}[taking]
	if err := writeSaying(root, quiet); err != nil {
		return err
	}
	if taking {
		took := false
		for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
			switch what, err := takeAgentFile(root, name); {
			case err != nil:
				return err
			case what == "moved":
				say("moved", name, ": what it said is now in docs/self/rules.md")
				took = true
			case what == "removed":
				say("removed", name, ": it only pointed to AGENTS.md")
				took = true
			}
		}
		if took {
			if err := writeSaying(root, nil); err != nil {
				return err
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
		if err := ignore(root, "node_modules/", "npm's folder"); err != nil {
			return err
		}
	}
	problems, err := Problems(root)
	if err != nil {
		return err
	}
	for _, p := range problems {
		fmt.Println("✗", p)
	}
	return nil
}

// writeSaying writes charter's files, and says what it wrote and removed, and which of the repo's own it kept (but
// those in quiet).
func writeSaying(root string, quiet []string) error {
	changed, kept, err := Write(root, false)
	if err != nil {
		return err
	}
	for _, name := range changed {
		if gone, ok := strings.CutPrefix(name, "removed "); ok {
			say("removed", gone, ": not charter's any more")
		} else {
			say("wrote", name, "")
		}
	}
	for _, name := range kept {
		if !slices.Contains(quiet, name) {
			say("kept", name, ": yours (it does not carry charter's mark), so charter leaves it")
		}
	}
	if len(changed) == 0 {
		say("ok", "charter's files", ": every one is what this charter writes")
	}
	return nil
}

// ignore adds line to the repo's .gitignore unless it has it.
func ignore(root, line, why string) error {
	p := filepath.Join(root, ".gitignore")
	b, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) == line || strings.TrimSpace(l) == "/"+line {
			return nil
		}
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	say("changed", ".gitignore", ": "+line+" ("+why+")")
	return os.WriteFile(p, append(b, []byte(line+"\n")...), 0o644)
}

// takeAgentFile moves what a repo's own agent file says (one charter did not write, and more than a pointer) to the end
// of docs/self/rules.md, headings one level down and links made relative to it, and removes the file, so charter's
// takes its place.
func takeAgentFile(root, name string) (string, error) {
	p := filepath.Join(root, name)
	b, err := os.ReadFile(p)
	if err != nil {
		return "", nil
	}
	if charters, _ := marked(string(b)); charters {
		return "", nil
	}
	notes := strings.TrimSpace(string(b))
	said := notes != "" && notes != "@AGENTS.md"
	if said {
		moved := regexp.MustCompile(`(?m)^(#{1,5} )`).ReplaceAllString(notes, "#$1")
		moved = regexp.MustCompile(`\]\(((?:[^)#:/][^):]*|\./[^):]*))\)`).ReplaceAllString(moved, "](../../$1)")
		rules := filepath.Join(root, "docs", "self", "rules.md")
		before, _ := os.ReadFile(rules)
		if err := os.MkdirAll(filepath.Dir(rules), 0o755); err != nil {
			return "", err
		}
		text := strings.TrimRight(string(before), "\n") + "\n\n## What " + name + " said before charter init\n\nMoved here by `charter init`, as it was. Yours to edit: keep what is still true, where it belongs.\n\n" + moved + "\n"
		if err := os.WriteFile(rules, []byte(text), 0o644); err != nil {
			return "", err
		}
	}
	if err := os.Remove(p); err != nil {
		return "", err
	}
	if said {
		return "moved", nil
	}
	return "removed", nil
}
