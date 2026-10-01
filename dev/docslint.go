package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func init() {
	commands["docs-lint"] = command{"[-into <repo dir>]",
		"check docs/ for what can be checked without reading: front matter, links, the index, tasks and paths that don't exist", docsLint}
	anywhere["docs-lint"] = true
}

var (
	mdLink    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	mdHeading = regexp.MustCompile(`(?m)^#{1,6} +(.+?) *$`)
	miseRun   = regexp.MustCompile("mise run ([a-z][a-z0-9:-]*)")
	taskDef   = regexp.MustCompile(`(?m)^\[tasks\."?([a-z0-9:-]+)"?\]`)
	codeSpan  = regexp.MustCompile("`([^`\n]+)`")
	fenced    = regexp.MustCompile("(?s)```.*?```")
	gitTag    = regexp.MustCompile(`/v([0-9]|X\.)`) // api-go/v0.1.0 is a tag, not a path
	// A version of this repo's own releases written into a page: @v1.2.3 after a module of ours, or a
	// link to one tagged release.
	pinnedVersion = regexp.MustCompile(`(orpc-api[A-Za-z0-9/_-]*@v[0-9]+\.[0-9]+\.[0-9]+|releases/(tag|download)/v[0-9][0-9A-Za-z.-]*)`)
	// A code span that is a path into the repo: one of its top-level entries, then a slash.
	repoPath = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./*<>{}-]*$`)
)

// docsLint finds what is wrong with docs/ that a program can see. What needs judgement (is the
// page true, clear, complete) is for a reader: mise run docs:review hands that to Claude.
func docsLint(args []string) error {
	into := "."
	flags("docs-lint", args, func(f *flag.FlagSet) { f.StringVar(&into, "into", into, "the repo whose docs/ to check") })
	if !filepath.IsAbs(into) && into != "." {
		into = filepath.Join(started, into)
	}
	docs := filepath.Join(into, "docs")
	var pages []string
	err := filepath.WalkDir(docs, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") && !strings.Contains(path, "/_") {
			pages = append(pages, path)
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("no docs/ in %s", into)
	}
	sort.Strings(pages)
	tasks := map[string]bool{}
	if mise, err := os.ReadFile(filepath.Join(into, "mise.toml")); err == nil {
		for _, m := range taskDef.FindAllStringSubmatch(string(mise), -1) {
			tasks[m[1]] = true
		}
	}
	top := map[string]bool{}
	if entries, err := os.ReadDir(into); err == nil {
		for _, e := range entries {
			top[e.Name()] = true
		}
	}
	// Every page must be reachable: linked from at least one other page.
	linked := map[string]bool{}
	for _, page := range pages {
		raw, _ := os.ReadFile(page)
		for _, m := range mdLink.FindAllStringSubmatch(string(raw), -1) {
			if target, _, _ := strings.Cut(m[1], "#"); target != "" && !strings.Contains(target, "://") {
				if to := filepath.Join(filepath.Dir(page), target); to != page {
					linked[to] = true
				}
			}
		}
	}

	var problems []string
	say := func(page, format string, a ...any) {
		rel, _ := filepath.Rel(into, page)
		problems = append(problems, rel+": "+fmt.Sprintf(format, a...))
	}
	for _, page := range pages {
		raw, err := os.ReadFile(page)
		if err != nil {
			return err
		}
		text := string(raw)
		if !strings.HasPrefix(text, "---\n") || !strings.Contains(strings.SplitN(text, "\n---", 2)[0], "title:") || !strings.Contains(strings.SplitN(text, "\n---", 2)[0], "nav_order:") {
			say(page, "no front matter with title and nav_order (the sidebar needs them)")
		}
		if filepath.Base(page) == "README.md" && filepath.Dir(page) == docs && !strings.Contains(strings.SplitN(text, "\n---", 2)[0], "permalink: /") {
			say(page, "the start page needs `permalink: /` in its front matter, or the site has no home page")
		}
		if strings.Contains(text, "{{") || strings.Contains(text, "{%") {
			say(page, "two curly braces together, or a curly brace and a percent sign: Jekyll reads those as template code")
		}
		rel, _ := filepath.Rel(docs, page)
		if rel != "README.md" && !linked[page] {
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
				if !exists(file) {
					say(page, "link to %s: no such file", m[1])
					continue
				}
			}
			if anchor != "" && strings.HasSuffix(file, ".md") && !hasAnchor(file, anchor) {
				say(page, "link to %s: no heading makes the anchor #%s", m[1], anchor)
			}
		}
		// Plans and findings describe what isn't built or what once was: only their links are checked.
		// writing.md is the same page in every repo, with example names of its own.
		if strings.Contains(rel, "plans/") || rel == "findings.md" || rel == "writing.md" {
			continue
		}
		for _, m := range pinnedVersion.FindAllString(text, -1) {
			say(page, "`%s`: a hard-coded release version goes stale. Use @latest, releases/latest, or the placeholder vX.Y.Z", m)
		}
		if len(tasks) > 0 {
			for _, m := range miseRun.FindAllStringSubmatch(text, -1) {
				if !tasks[m[1]] {
					say(page, "`mise run %s`: no such task in mise.toml", m[1])
				}
			}
		}
		for _, m := range codeSpan.FindAllStringSubmatch(fenced.ReplaceAllString(text, ""), -1) {
			span := strings.TrimRight(m[1], ":,.")
			first, _, _ := strings.Cut(span, "/")
			if !repoPath.MatchString(span) || !top[first] || strings.ContainsAny(span, "*<>{}") || strings.Contains(span, "...") || gitTag.MatchString(span) {
				continue
			}
			if !exists(filepath.Join(into, span)) && !ignored(into, span) {
				say(page, "`%s`: no such path in the repo", span)
			}
		}
	}
	if len(problems) == 0 {
		fmt.Printf("docs: %d pages, nothing a program can fault\n", len(pages))
		return nil
	}
	fmt.Println(strings.Join(problems, "\n"))
	return fmt.Errorf("docs: %d problems in %d pages", len(problems), len(pages))
}

// ignored reports whether git ignores the path: a build's output (sdk/out/) is a real place that a
// fresh checkout doesn't have yet.
func ignored(repo, path string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", path)
	cmd.Dir = repo
	return cmd.Run() == nil
}

// hasAnchor reports whether a heading in file makes this anchor, the way GitHub and Jekyll do:
// lower case, spaces to hyphens, punctuation dropped.
func hasAnchor(file, anchor string) bool {
	raw, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	drop := regexp.MustCompile("[^a-z0-9 _-]")
	for _, m := range mdHeading.FindAllStringSubmatch(string(raw), -1) {
		slug := strings.ReplaceAll(drop.ReplaceAllString(strings.ToLower(m[1]), ""), " ", "-")
		if slug == anchor {
			return true
		}
	}
	return false
}

var errNoClaude = errors.New("docs-review needs the claude command (Claude Code) on PATH")
