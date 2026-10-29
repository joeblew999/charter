package repo

import (
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// review is what charter docs -review asks Claude to do, with what the lint found in place of __LINT__.
//
//go:embed review.md
var review string

var (
	mdLink    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	mdHeading = regexp.MustCompile(`(?m)^#{1,6} +(.+?) *$`)
	codeSpan  = regexp.MustCompile("`([^`\n]+)`")
	fenced    = regexp.MustCompile("(?s)```.*?```")
	gitTag    = regexp.MustCompile(`/v([0-9]|X\.)`) // go/lib/v0.1.0 is a tag, not a path
	// A release's version, pinned where it goes stale. An install command (releases/download/vX.Y.Z/…,
	// github:joeblew999/charter@X.Y.Z) is not: charter release writes each one to the version it releases.
	pinnedVersion = regexp.MustCompile(`(joeblew999/charter[A-Za-z0-9/_-]*@v[0-9]+\.[0-9]+\.[0-9]+|releases/tag/(?:ts-)?v[0-9][0-9A-Za-z.-]*)`)
	repoPath      = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./*<>{}-]*$`)
	npmRun        = regexp.MustCompile(`npm run ([a-z][a-z0-9:-]*)`)
	charterCmd    = regexp.MustCompile("`(?:go tool )?charter ([a-z][a-z-]*)")
	miseTask      = regexp.MustCompile(`mise run (charter:[a-z][a-z-]*)`)
	antiSlug      = regexp.MustCompile(`[^a-z0-9 _-]`)
)

// pagesIn are the Markdown pages under dir, but those of the site's own folders (_*) and of packages.
func pagesIn(dir string) []string {
	var out []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && p != dir && (strings.HasPrefix(d.Name(), "_") || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") && !strings.HasPrefix(d.Name(), "_") {
			out = append(out, p)
		}
		return nil
	})
	slices.Sort(out)
	return out
}

// scriptsOf are the npm scripts a page may run: those of every package.json git tracks.
func scriptsOf(root string) map[string]bool {
	out := map[string]bool{}
	files, _ := exec.Command("git", "-C", root, "ls-files", "--", "*package.json", "package.json").Output()
	for _, f := range strings.Fields(string(files)) {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			continue
		}
		var pkg struct{ Scripts map[string]string }
		json.Unmarshal(b, &pkg)
		for name := range pkg.Scripts {
			out[name] = true
		}
	}
	return out
}

// tasksOf are the mise tasks a page may run: those of the repo's .mise/tasks/charter.toml, and of every project's.
func tasksOf(root string) map[string]bool {
	out := map[string]bool{}
	files, _ := exec.Command("git", "-C", root, "ls-files", "--", "*.mise/tasks/charter.toml", ".mise/tasks/charter.toml").Output()
	for _, f := range strings.Fields(string(files)) {
		var tasks map[string]any
		if _, err := toml.DecodeFile(filepath.Join(root, f), &tasks); err == nil {
			for name := range tasks {
				out[name] = true
			}
		}
	}
	return out
}

// hasAnchor is whether a heading of file makes the anchor, as GitHub and Jekyll do: lower case, spaces to hyphens,
// punctuation dropped.
func hasAnchor(file, anchor string) bool {
	b, _ := os.ReadFile(file)
	for _, m := range mdHeading.FindAllStringSubmatch(string(b), -1) {
		if strings.ReplaceAll(antiSlug.ReplaceAllString(strings.ToLower(m[1]), ""), " ", "-") == anchor {
			return true
		}
	}
	return false
}

// ignored is whether git ignores the path, or it is in a submodule: a real place a fresh checkout doesn't have yet.
func ignored(root, path string) bool {
	if exec.Command("git", "-C", root, "check-ignore", "-q", path).Run() == nil {
		return true
	}
	out, err := exec.Command("git", "-C", root, "config", "-f", ".gitmodules", "--get-regexp", `\.path$`).Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if _, sub, ok := strings.Cut(line, " "); ok && (path == sub || strings.HasPrefix(path, sub+"/")) {
			return true
		}
	}
	return false
}

// Lint is what a program can see wrong in the docs of the repo at root ([github] docs: docs, or the repo's top), one
// line each, and how many pages it read.
func Lint(root string) ([]string, int, error) {
	s, err := settings(root)
	if err != nil {
		return nil, 0, err
	}
	docs := filepath.Join(root, s.GitHub.DocsFolder())
	if !exists(filepath.Join(docs, "self")) && !exists(filepath.Join(docs, folder)) {
		return nil, 0, nil
	}
	pages := pagesIn(docs)
	scripts, tasks := scriptsOf(root), tasksOf(root)
	commands := map[string]bool{"help": true}
	for _, c := range Commands {
		commands[c.Name] = true
	}
	for _, c := range Extra {
		commands[c] = true
	}
	// A span is a path of the repo's when its first folder is one at the repo's top that git tracks: what is on this
	// disk and not in git (an agent's .claude/, a build's folders) would make one checkout's answer differ from
	// another's.
	top := map[string]bool{}
	tracked, _ := exec.Command("git", "-C", root, "ls-files").Output()
	for _, f := range strings.Split(string(tracked), "\n") {
		if first, _, _ := strings.Cut(f, "/"); first != "" {
			top[first] = true
		}
	}
	// Every page must be reachable: linked from at least one other page.
	linked := map[string]bool{}
	for _, page := range pages {
		b, _ := os.ReadFile(page)
		for _, m := range mdLink.FindAllStringSubmatch(string(b), -1) {
			target, _, _ := strings.Cut(m[1], "#")
			if target != "" && !strings.Contains(target, "://") {
				linked[filepath.Join(filepath.Dir(page), target)] = true
			}
		}
	}
	var problems []string
	say := func(page, problem string) {
		rel, _ := filepath.Rel(root, page)
		problems = append(problems, filepath.ToSlash(rel)+": "+problem)
	}
	for _, page := range pages {
		b, _ := os.ReadFile(page)
		text := string(b)
		front, _, _ := strings.Cut(text, "\n---")
		rel, _ := filepath.Rel(docs, page)
		rel = filepath.ToSlash(rel)
		if !strings.HasPrefix(text, "---\n") || !strings.Contains(front, "title:") || !strings.Contains(front, "nav_order:") {
			say(page, "no front matter with title and nav_order (the sidebar needs them)")
		}
		if rel == "self/README.md" && !strings.Contains(front, "permalink: /") {
			say(page, "the start page needs `permalink: /` in its front matter, or the site has no home page")
		}
		if strings.Contains(text, "{{") || strings.Contains(text, "{%") {
			say(page, "two curly braces together, or a curly brace and a percent sign: Jekyll reads those as template code")
		}
		// The two folders' first pages, the repo's own and charter's: the sidebar and AGENTS.md reach them.
		if rel != "self/README.md" && rel != folder+"/README.md" && !linked[page] {
			say(page, "no other page links to it: add it to its section's table (or the home page's)")
		}
		// Links and paths are read from the prose: an example inside code is not a link.
		prose := codeSpan.ReplaceAllString(fenced.ReplaceAllString(text, ""), "")
		for _, m := range mdLink.FindAllStringSubmatch(prose, -1) {
			target, anchor, _ := strings.Cut(m[1], "#")
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			file := page
			if target != "" {
				file = filepath.Join(filepath.Dir(page), target)
			}
			if target != "" && !exists(file) {
				say(page, "link to "+m[1]+": no such file")
				continue
			}
			if anchor != "" && strings.HasSuffix(file, ".md") && !hasAnchor(file, anchor) {
				say(page, "link to "+m[1]+": no heading makes the anchor #"+anchor)
			}
		}
		// charter's pages are the same in every repo, with example names of their own: only their links are checked.
		if strings.HasPrefix(rel, folder+"/") {
			continue
		}
		for _, m := range pinnedVersion.FindAllString(text, -1) {
			say(page, "`"+m+"`: a release version goes stale here. Use releases/latest, or the placeholder vX.Y.Z")
		}
		for _, m := range npmRun.FindAllStringSubmatch(text, -1) {
			if !scripts[m[1]] {
				say(page, "`npm run "+m[1]+"`: no such script in a package.json")
			}
		}
		for _, m := range charterCmd.FindAllStringSubmatch(text, -1) {
			if !commands[m[1]] {
				say(page, "`charter "+m[1]+"`: charter has no such command (charter help lists them)")
			}
		}
		for _, m := range miseTask.FindAllStringSubmatch(text, -1) {
			if len(tasks) > 0 && !tasks[m[1]] {
				say(page, "`mise run "+m[1]+"`: no such task in .mise/tasks/charter.toml")
			}
		}
		for _, m := range codeSpan.FindAllStringSubmatch(fenced.ReplaceAllString(text, ""), -1) {
			span := strings.TrimRight(m[1], ":,.")
			first, _, _ := strings.Cut(span, "/")
			if !repoPath.MatchString(span) || !top[first] || strings.ContainsAny(span, "*<>{}") || strings.Contains(span, "...") || gitTag.MatchString(span) {
				continue
			}
			if !exists(filepath.Join(root, span)) && !ignored(root, span) {
				say(page, "`"+span+"`: no such path in the repo")
			}
		}
	}
	return problems, len(pages), nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Extra are charter's commands that are no step of a developer's (Commands leaves them out): what charter's own files
// and CI run.
var Extra = []string{"serve", "with", "local", "token", "tail", "bench", "builds", "tasks", "skill", "workflow", "test", "version"}

// Docs runs charter docs [-review [-print]]: what a program can see wrong in the repo's docs (front matter, links and
// their anchors, pages nothing links to, commands, scripts and paths that don't exist, release versions that go
// stale); with -review, Claude reads them for what needs judgement: is the page true, clear, complete.
func Docs(args []string) {
	fs := flag.NewFlagSet("charter docs", flag.ExitOnError)
	reviewing := fs.Bool("review", false, "have Claude (the claude command) review the docs, with what the lint found")
	print := fs.Bool("print", false, "with -review: print the prompt, run nothing")
	fs.Parse(args)
	r := root()
	problems, pages, err := Lint(r)
	check(err)
	report := fmt.Sprintf("docs: %d pages, nothing a program can fault", pages)
	if len(problems) > 0 {
		report = strings.Join(problems, "\n") + fmt.Sprintf("\ndocs: %d problems in %d pages", len(problems), pages)
	}
	if !*reviewing {
		fmt.Println(report)
		if len(problems) > 0 {
			os.Exit(1)
		}
		return
	}
	prompt := strings.Replace(review, "__LINT__", report, 1)
	if *print {
		fmt.Println(prompt)
		return
	}
	fmt.Println("Claude is reviewing docs/ (it prints its report when it is done; watch `git status` for its edits)")
	claude := exec.Command("claude", "-p", prompt, "--permission-mode", "acceptEdits", "--allowedTools", "Read,Edit,Write,Glob,Grep,Bash(charter docs),Bash(go tool charter docs),Bash(git status:*),Bash(git diff:*),Bash(ls:*)")
	claude.Dir, claude.Stdout, claude.Stderr = r, os.Stdout, os.Stderr
	if err := claude.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			check(fmt.Errorf("claude stopped (%d)", exit.ExitCode()))
		}
		check(errors.New("docs -review needs the claude command (Claude Code) on PATH"))
	}
}
