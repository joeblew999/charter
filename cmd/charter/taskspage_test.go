package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// The tasks page: grouped by source (the repo's mise.toml, then its includes as listed, a git
// include named without its ref), in the order the tasks are written and not mise's order by name;
// no hidden task, none of a config outside the repo; -only and -not choose sources; a description
// cannot break its table or the site's renderer; each task's detail is mise's, without Depends.
func TestTasksPage(t *testing.T) {
	root := filepath.FromSlash("/work/field-notes")
	cache := filepath.FromSlash("/home/me/.cache/mise/remote-git-tasks-cache/abc123/tasks/repo/")
	in := func(file string) string { return filepath.Join(root, filepath.FromSlash(file)) }
	files := map[string]string{
		in("mise.toml"):        "[tools]\nnode = \"26\"\n[tasks.test]\nrun = \"x\"\n[tasks.\"docs:lint\"]\nrun = \"mine\"\n",
		in("tasks.toml"):       "[\"site:new\"]\nrun = \"x\"\n[step]\nhide = true\n[\"site:start\"]\nrun = \"x\"\n[alpha]\nrun = \"x\"\n",
		cache + "repo.toml":    "[\"repo\"]\nrun = \"x\"\n[\"issues\"]\nrun = \"x\"\n",
		cache + "docs.toml":    "[\"docs:setup\"]\nrun = \"x\"\n",
		in("tools/build.toml"): "[build]\nrun = \"x\"\n",
	}
	read := func(file string) ([]byte, error) {
		if content, ok := files[file]; ok {
			return []byte(content), nil
		}
		return nil, errors.New("no such file")
	}
	includes := []string{"tasks.toml", "git::https://github.com/joeblew999/charter.git//tasks/repo?ref=v1.2.3", "tools"}
	tasks := []miseTask{ // as mise lists them: by name
		{Name: "alpha", Description: "Last in its file", Source: in("tasks.toml")},
		{Name: "build", Description: "Build", Source: in("tools/build.toml")},
		{Name: "docs:lint", Description: "This repo's own lint", Source: in("mise.toml")},
		{Name: "docs:setup", Description: "Write the site", Source: cache + "docs.toml"},
		{Name: "global", Description: "The user's own", Source: filepath.FromSlash("/home/me/.config/mise/config.toml"), Global: true},
		{Name: "issues", Description: "The open issues", Source: cache + "repo.toml"},
		{Name: "above", Description: "Of a folder above", Source: filepath.FromSlash("/work/mise.toml")},
		{Name: "repo", Description: "Keep the repo", Source: cache + "repo.toml"},
		{Name: "site:new", Description: "Make a site: mise run site:new -- <template> | {{name}}", Source: in("tasks.toml")},
		{Name: "site:start", Description: "Start it", Source: in("tasks.toml")},
		{Name: "step", Description: "A step", Source: in("tasks.toml"), Hide: true},
		{Name: "test", Description: "Test", Source: in("mise.toml")},
	}
	details := "## `alpha`\n\n- **Usage:** `alpha`\n\nLast in its file\n\n## `site:new`\n\n- Depends: step\n\n- **Usage:** `site:new [template]`\n\n### Arguments\n- **`[template]`** — which\n\n### Flags\n- **`--live`** — there\n\n## `step`\n\nA step\n"
	page, err := tasksPage("Tasks", root, includes, tasks, details, nil, nil, read)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(page, "# Tasks\n\nRun one with `mise run <task>`") {
		t.Errorf("the page starts:\n%.80s", page)
	}
	var order []string
	for _, line := range strings.Split(page[:strings.Index(page, "\n## Each task")], "\n") {
		if strings.HasPrefix(line, "## ") {
			order = append(order, line)
		} else if m := taskBlock.FindStringSubmatch("## " + strings.TrimPrefix(strings.TrimPrefix(line, "| ["), "| ")); strings.HasPrefix(line, "| [`") && m != nil {
			order = append(order, m[1])
		}
	}
	want := "## From `mise.toml` test docs:lint ## From `tasks.toml` site:new site:start alpha ## From `joeblew999/charter//tasks/repo` docs:setup repo issues ## From `tools` build"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("the tasks, in order:\n got %s\nwant %s", got, want)
	}
	for _, want := range []string{
		"| [`site:new`](#sitenew) | Make a site: mise run site:new \\-\\- &lt;template&gt; \\| &#123;&#123;name}} |\n",
		"\n### `site:new`\n\n- **Usage:** `site:new [template]`\n\n**Arguments**\n- **`[template]`** — which\n\n**Flags**\n- **`--live`** — there\n",
		"\n### `test`\n",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("no %q in:\n%s", want, page)
		}
	}
	for _, gone := range []string{"Depends", "step`", "global", "above", "{{", "v1.2.3", "abc123"} {
		if strings.Contains(page, gone) {
			t.Errorf("the page has %q", gone)
		}
	}
	// One source, by a part of its name; a source left out; a name no source has.
	only, err := tasksPage("Tasks", root, includes, tasks, details, []string{"tasks.toml"}, nil, read)
	if err != nil || strings.Contains(only, "`repo`") || strings.Contains(only, "`test`") || !strings.Contains(only, "### `alpha`") {
		t.Errorf("-only tasks.toml: %v\n%s", err, only)
	}
	not, err := tasksPage("Tasks", root, includes, tasks, details, nil, []string{"charter", "mise.toml"}, read)
	if err != nil || strings.Contains(not, "`repo`") || strings.Contains(not, "`test`") || !strings.Contains(not, "### `build`") {
		t.Errorf("-not charter,mise.toml: %v\n%s", err, not)
	}
	if _, err := tasksPage("Tasks", root, includes, tasks, details, []string{"nowhere"}, nil, read); err == nil || !strings.Contains(err.Error(), "the sources are mise.toml, tasks.toml, joeblew999/charter//tasks/repo, tools") {
		t.Errorf("-only nowhere: %v", err)
	}
	// What charter docs makes of it: a page with a title and no template code.
	if _, err := generatedPageText(t.TempDir(), "docs", generatedPage{Page: "tasks.md", Run: "charter docs-tasks"}, 0, page); err != nil {
		t.Errorf("as a generated page: %v", err)
	}
}
