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
	"sort"
	"strings"
)

// `charter docs-tasks` prints a repo's tasks as a docs page, from what mise knows of them: every
// repo that uses charter has its tasks in mise, so one command writes the page for all of them.
// Listed in docs/_generated.toml it is a generated page like any other: committed, in the site and
// in llms.txt, and stale when a task changes (charter docs -check).
//
// The tasks are grouped by where they come from (the repo's mise.toml, then each entry of its
// [task_config] includes), and within a group they keep the order they are written in: mise lists
// them by name, and the order in the file is the author's.

func init() {
	commands["docs-tasks"] = command{"[-only <source,...>] [-not <source,...>] [-title <text>]",
		"print the repo's mise tasks as a docs page, for docs/_generated.toml: grouped by the file or include they come from, in the order they are written, each with its description, arguments and flags; -only and -not choose the sources", docsTasks}
	anywhere["docs-tasks"] = true
}

// miseTask is a task as `mise tasks ls --json` gives it.
type miseTask struct {
	Name, Description, Source string
	Hide, Global              bool
}

func docsTasks(args []string) error {
	var only, not, title string
	flags("docs-tasks", args, func(f *flag.FlagSet) {
		f.StringVar(&only, "only", "", "only the tasks from these sources, comma-separated: a source is named as the page's headings name it (mise.toml, tasks.toml, owner/repo//folder), and a part of the name is enough")
		f.StringVar(&not, "not", "", "leave out the tasks from these sources, named the same way")
		f.StringVar(&title, "title", "Tasks", "the page's title")
	})
	root, err := output(started, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return errors.New("docs-tasks runs in a git repo")
	}
	root = filepath.FromSlash(root)
	config, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		return fmt.Errorf("docs-tasks reads the repo's tasks from mise: no mise.toml at %s", root)
	}
	listed, err := output(root, "mise", "tasks", "ls", "--json")
	if err != nil {
		return errors.New("docs-tasks: mise tasks ls --json failed")
	}
	var tasks []miseTask
	if err := json.Unmarshal([]byte(listed), &tasks); err != nil {
		return fmt.Errorf("docs-tasks: what mise tasks ls --json printed: %w", err)
	}
	details, err := output(root, "mise", "generate", "task-docs", "--style", "detailed")
	if err != nil {
		return errors.New("docs-tasks: mise generate task-docs failed")
	}
	page, err := tasksPage(title, root, includesOf(string(config)), tasks, details, commaList(only), commaList(not), os.ReadFile)
	if err != nil {
		return err
	}
	fmt.Print(page)
	return nil
}

// commaList is a comma-separated flag's values.
func commaList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// includesOf are the entries of a mise.toml's [task_config] includes, in the order written.
func includesOf(config string) []string {
	var includes []string
	for _, m := range includeDef.FindAllStringSubmatch(config, -1) {
		for _, include := range quoted.FindAllStringSubmatch(m[1], -1) {
			includes = append(includes, include[1])
		}
	}
	return includes
}

var (
	// An include from a git repo: git::https://github.com/<owner>/<repo>.git//<path>?ref=<ref>
	gitInclude = regexp.MustCompile(`^git::https?://[^/]+/(.+?)(?:\.git)?//([^?]+)`)
	// Where mise keeps a git include it fetched: .../remote-git-tasks-cache/<hash>/<path in the repo>
	miseGitCache = regexp.MustCompile(`remote-git-tasks-cache/[^/]+/(.+)$`)
	// A task's table in a task file, as mise.toml writes it ([tasks.x], [tasks."x:y"]) and as an
	// included file does ([x], ["x:y"]).
	taskTable = regexp.MustCompile(`(?m)^\[(?:tasks\.)?"?([A-Za-z0-9_:.-]+)"?\]`)
	// A task's block in what mise generate task-docs prints.
	taskBlock = regexp.MustCompile("(?m)^## `([^`]+)`")
)

// tasksPage is the page: its title, how to run a task, then for each source a table of its tasks
// and what each does, then every task with its usage, arguments and flags as mise documents them.
// tasks are mise's (the hidden ones and those of a config outside the repo are left out), details
// is what mise generate task-docs --style detailed printed, and read reads a task file for the
// order its tasks are written in.
func tasksPage(title, root string, includes []string, tasks []miseTask, details string, only, not []string, read func(string) ([]byte, error)) (string, error) {
	type group struct {
		name  string
		tasks []miseTask
	}
	// The sources, in order: the repo's own mise.toml, then its includes as listed.
	names := []string{"mise.toml"}
	for _, include := range includes {
		names = append(names, sourceName(include))
	}
	groups := map[string]*group{}
	rank := map[string]int{} // a task's place: its file among the group's, then its place in the file
	order := map[string][]string{}
	for _, task := range tasks {
		name, ok := sourceOf(root, includes, task.Source)
		if task.Hide || task.Global || !ok {
			continue
		}
		if groups[name] == nil {
			groups[name] = &group{name: name}
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
		groups[name].tasks = append(groups[name].tasks, task)
		if _, seen := order[task.Source]; !seen {
			order[task.Source] = nil
			if content, err := read(task.Source); err == nil {
				for _, m := range taskTable.FindAllStringSubmatch(string(content), -1) {
					order[task.Source] = append(order[task.Source], m[1])
				}
			}
		}
		at := slices.Index(order[task.Source], task.Name)
		if at < 0 {
			at = len(order[task.Source]) // a task that is a file of its own: after the written ones
		}
		rank[task.Name] = at
	}
	chosen := func(name string) bool {
		match := func(parts []string) bool {
			return slices.ContainsFunc(parts, func(part string) bool { return strings.Contains(name, part) })
		}
		return (len(only) == 0 || match(only)) && !match(not)
	}
	for _, part := range append(slices.Clone(only), not...) {
		if !slices.ContainsFunc(names, func(name string) bool { return groups[name] != nil && strings.Contains(name, part) }) {
			var have []string
			for _, name := range names {
				if groups[name] != nil {
					have = append(have, name)
				}
			}
			return "", fmt.Errorf("docs-tasks: no tasks come from a source named %q: the sources are %s", part, strings.Join(have, ", "))
		}
	}
	blocks := map[string]string{}
	at := taskBlock.FindAllStringSubmatchIndex(details, -1)
	for i, m := range at {
		end := len(details)
		if i+1 < len(at) {
			end = at[i+1][0]
		}
		blocks[details[m[2]:m[3]]] = details[m[1]:end]
	}

	var page, each strings.Builder
	page.WriteString("# " + title + "\n\nRun one with `mise run <task>`; its arguments and flags go after `--`. `mise tasks` lists them, and `mise run <task> --help` shows one.\n")
	count := 0
	for _, name := range names {
		g := groups[name]
		if g == nil || !chosen(name) {
			continue
		}
		sort.SliceStable(g.tasks, func(i, j int) bool {
			a, b := g.tasks[i], g.tasks[j]
			if a.Source != b.Source {
				return a.Source < b.Source
			}
			if rank[a.Name] != rank[b.Name] {
				return rank[a.Name] < rank[b.Name]
			}
			return a.Name < b.Name
		})
		page.WriteString("\n## From `" + name + "`\n\n| Task | What it does |\n|---|---|\n")
		for _, task := range g.tasks {
			count++
			page.WriteString("| [`" + task.Name + "`](#" + anchorOf("`"+task.Name+"`") + ") | " + cellText(task.Description) + " |\n")
			each.WriteString("\n### `" + task.Name + "`\n\n" + taskDetail(blocks[task.Name]) + "\n")
		}
	}
	if count == 0 {
		return "", errors.New("docs-tasks: no tasks to list (hidden tasks and those of a mise config outside the repo are left out)")
	}
	page.WriteString("\n## Each task\n" + each.String())
	// The site's renderer reads two curly braces, or one and a percent sign, as template code.
	return strings.NewReplacer("{{", "&#123;&#123;", "{%", "&#123;%").Replace(page.String()), nil
}

// sourceName is how the page names an include: a file or folder of the repo as it is written, a
// git include as owner/repo//folder (no ref: the page does not change with every release).
func sourceName(include string) string {
	if m := gitInclude.FindStringSubmatch(include); m != nil {
		return m[1] + "//" + strings.TrimSuffix(m[2], "/")
	}
	path := strings.TrimSuffix(filepath.ToSlash(include), "/")
	if filepath.IsAbs(include) {
		// A folder outside the repo (a checkout of charter): its last two names, not this machine's path.
		parts := strings.Split(path, "/")
		path = strings.Join(parts[max(0, len(parts)-2):], "/")
	}
	return path
}

// sourceOf is the source a task's file belongs to, by the name the page gives it: the include it is
// or is under, else its path in the repo. A file outside the repo that no include accounts for (a
// mise config above the repo, or the user's own) is not the repo's: false.
func sourceOf(root string, includes []string, file string) (string, bool) {
	slashed := filepath.ToSlash(file)
	inCache := miseGitCache.FindStringSubmatch(slashed)
	rel, err := filepath.Rel(root, file)
	inRepo := err == nil && !strings.HasPrefix(rel, "..") && inCache == nil
	rel = filepath.ToSlash(rel)
	for _, include := range includes {
		if m := gitInclude.FindStringSubmatch(include); m != nil {
			path := strings.TrimSuffix(m[2], "/")
			if inCache != nil && (inCache[1] == path || strings.HasPrefix(inCache[1], path+"/")) {
				return sourceName(include), true
			}
			continue
		}
		path := strings.TrimSuffix(filepath.ToSlash(include), "/")
		if filepath.IsAbs(include) && (slashed == path || strings.HasPrefix(slashed, path+"/")) {
			return sourceName(include), true
		}
		if inRepo && (rel == path || strings.HasPrefix(rel, path+"/")) {
			return path, true
		}
	}
	switch {
	case inRepo:
		return rel, true
	case inCache != nil:
		return inCache[1], true
	}
	return "", false
}

// anchorOf is the anchor GitHub and the docs site give a heading (as docs-lint's hasAnchor reads it).
func anchorOf(heading string) string {
	return strings.ReplaceAll(regexp.MustCompile("[^a-z0-9 _-]").ReplaceAllString(strings.ToLower(heading), ""), " ", "-")
}

// cellText is text as a table cell holds it: an angle bracket would start a tag, a bar end the
// cell, and two dashes become one long one.
func cellText(text string) string {
	return strings.NewReplacer("<", "&lt;", ">", "&gt;", "|", `\|`, "--", `\-\-`, "\n", " ").Replace(strings.TrimSpace(text))
}

// taskDetail is a task's block of mise's own docs under the page's heading for it: without the
// tasks it depends on (hidden steps are not something to run), and with its Arguments and Flags
// headings as bold lines, so the page's outline is its tasks.
func taskDetail(block string) string {
	var lines []string
	for _, line := range strings.Split(block, "\n") {
		switch {
		case strings.HasPrefix(line, "- Depends:"):
			continue
		case line == "### Arguments" || line == "### Flags":
			line = "**" + strings.TrimPrefix(line, "### ") + "**"
		}
		lines = append(lines, line)
	}
	text := regexp.MustCompile(`\n{3,}`).ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
	return strings.TrimSpace(text)
}
