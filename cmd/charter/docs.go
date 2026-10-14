package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func init() {
	commands["docs"] = command{"[-into <repo dir>] [-check]",
		"write the pages docs/_generated.toml lists, each from its command (charter's own pages and the site's files are the kit's: charter adopt); -check fails if one is stale", docs}
	commands["docs-review"] = command{"[-print]",
		"have Claude bring docs/ up to date and into line with docs/charter/writing.md (-print: only show the prompt)", docsReviewRun}
	anywhere["docs"], anywhere["docs-review"] = true, true
}

// The prompt docs-review hands Claude: the same for every repo.
//
//go:embed review.md
var docsReview string

// docsUsing is the page in charter's folder that says how to use charter, written from the tool
// itself: the same in every repo, at the release its mise.toml pins, so what an agent is told matches
// the tool it has. AGENTS.md loads it with the rules.
func docsUsing() string {
	cell := strings.NewReplacer("|", `\|`, "\n", " ").Replace
	var page strings.Builder
	page.WriteString("---\ntitle: Using charter\nnav_order: 2\nparent: How this repo is kept\n---\n\n# Using charter\n\n" +
		"Written by `charter adopt` from the tool itself (the same page in every repo that uses it, at the release its `mise.toml` pins): don't edit it here, change the commands in `cmd/charter/` in [charter](https://github.com/joeblew999/charter).\n\n" +
		"charter is the tool this repo is kept with. mise installs the release `mise.toml` pins, and puts it on the path of every task.\n\n" +
		"| To | Run |\n|---|---|\n" +
		"| See what there is to run | `mise tasks` |\n" +
		"| Run a task | `mise run <task>` |\n" +
		"| Run the tool | `charter <command>`; from anywhere, `mise x " + toolRelease + " -- charter <command>` |\n" +
		"| Move to another release | `mise x " + toolRelease + "@X.Y.Z -- charter upgrade`: the pin and the includes together |\n" +
		"| Read more | [charter's docs](https://joeblew999.github.io/charter/); for machines, [llms.txt](https://joeblew999.github.io/charter/llms.txt) |\n\n" +
		"## The commands\n\nThose marked * work in any repo; the others in a project, a folder with a `mise.toml` beside a `fern/` folder.\n\n" +
		"| Command | What it does |\n|---|---|\n")
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		star := ""
		if anywhere[name] {
			star = " *"
		}
		usage := strings.TrimSpace(name + " " + commands[name].usage)
		fmt.Fprintf(&page, "| `%s`%s | %s |\n", cell(usage), star, cell(commands[name].help))
	}
	return page.String()
}

// docs writes the pages the repo's docs/_generated.toml lists, each from what its command prints.
// Charter's own pages and the docs site's files are the kit's: charter adopt writes them. With -check
// nothing is written, and it fails if a page is stale.
func docs(args []string) error {
	into, check := ".", false
	flags("docs", args, func(f *flag.FlagSet) {
		f.StringVar(&into, "into", into, "the repo to write into (its docs/ folder)")
		f.BoolVar(&check, "check", false, "write nothing; fail if a page is stale")
	})
	config, err := readCharterToml(into)
	if err != nil {
		return err
	}
	list, err := readGenerated(filepath.Join(into, filepath.FromSlash(config.Docs)))
	if err != nil {
		return err
	}
	pages := map[string][]byte{}
	if err := addGenerated(pages, into, config.Docs, list); err != nil {
		return err
	}
	stale, err := syncFiles(into, pages, check)
	if err != nil {
		return err
	}
	run := map[string]string{}
	for _, g := range list {
		run[filepath.ToSlash(filepath.Join(config.Docs, filepath.FromSlash(g.Page)))] = g.Run
	}
	for _, path := range stale {
		if check {
			fmt.Fprintf(os.Stderr, "%s is stale: `%s` now prints something else (mise run docs:setup)\n", path, run[path])
		} else {
			fmt.Println("wrote", path)
		}
	}
	if check && len(stale) > 0 {
		return errors.New("a generated page is stale")
	}
	fmt.Printf("%d generated pages (%s), %d written\n", len(list), filepath.ToSlash(filepath.Join(config.Docs, generatedFile)), len(stale))
	return nil
}

// githubRepo is what gh repo view --json says about a repo.
type githubRepo struct {
	NameWithOwner, Name, Description, HomepageURL string
	DefaultBranchRef                              struct{ Name string }
	RepositoryTopics                              []struct{ Name string }
}

// addGenerated adds the generated pages to files (by their path in the repo at root, below the
// docs folder), each written from what its command prints, run at root.
func addGenerated(files map[string][]byte, root, docs string, generated []generatedPage) error {
	for i, g := range generated {
		args := splitCommand(g.Run)
		var out []byte
		if len(args) >= 2 && args[0] == "charter" && args[1] == "docs-tasks" {
			// charter's own page: written by this tool itself, not by whichever charter is on the path
			text, err := docsTasksPage(root, args[2:])
			if err != nil {
				return fmt.Errorf("generated page %s: %s: %w", g.Page, g.Run, err)
			}
			out = []byte(text)
		} else {
			cmd := exec.Command(args[0], args[1:]...)
			var stderr bytes.Buffer
			cmd.Dir, cmd.Stderr = root, &stderr
			var err error
			if out, err = cmd.Output(); err != nil {
				return fmt.Errorf("generated page %s: %s: %v\n%s", g.Page, g.Run, err, strings.TrimSpace(stderr.String()))
			}
		}
		page, err := generatedPageText(root, docs, g, i, string(out))
		if err != nil {
			return fmt.Errorf("generated page %s: %s: %w", g.Page, g.Run, err)
		}
		files[filepath.Join(docs, filepath.FromSlash(g.Page))] = []byte(page)
	}
	return nil
}

// generatedPageText is a generated page as it is committed: front matter, then what the command
// printed, with a line under its title that says where it comes from. The title is the output's
// first "# " heading; the parent is the title of its section's page (reference/x.md: reference.md);
// nav_order puts the generated pages after the written ones, in the listed order.
func generatedPageText(root, docs string, g generatedPage, index int, out string) (string, error) {
	out = strings.ReplaceAll(out, "\r\n", "\n")
	if strings.HasPrefix(out, "---\n") {
		return "", errors.New("it printed front matter: charter writes that, the command prints the page from its # title on")
	}
	heading := regexp.MustCompile(`(?m)^# +(.+?) *$`).FindStringSubmatchIndex(out)
	if heading == nil {
		return "", errors.New("it printed no # title")
	}
	title := out[heading[2]:heading[3]]
	front := "---\ntitle: " + yamlString(title) + "\nnav_order: " + strconv.Itoa(100+index) + "\n"
	if dir := filepath.Dir(filepath.FromSlash(g.Page)); dir != "." {
		section := filepath.Join(root, docs, dir+".md")
		parent := ""
		if raw, err := os.ReadFile(section); err == nil {
			if m := regexp.MustCompile(`(?m)^title: *(.+?) *$`).FindStringSubmatch(strings.SplitN(string(raw), "\n---", 2)[0]); m != nil {
				parent = strings.Trim(m[1], `"'`)
			}
		}
		if parent == "" {
			return "", fmt.Errorf("its parent is the section page %s, which has no title", filepath.ToSlash(filepath.Join(docs, dir+".md")))
		}
		front += "parent: " + yamlString(parent) + "\n"
	}
	note := "\n\nWritten by `charter docs` from what `" + g.Run + "` prints (docs/_generated.toml): don't edit, change the code that command reads, then `mise run docs:setup`."
	body := out[:heading[1]] + note + out[heading[1]:]
	return front + "---\n\n" + strings.TrimRight(body, "\n") + "\n", nil
}

// yamlString is s as a YAML string: JSON's quoting, which YAML reads.
func yamlString(s string) string {
	quoted, _ := json.Marshal(s)
	return string(quoted)
}

// splitCommand splits a command line into its program and arguments at spaces; "double quotes"
// hold one with spaces. No shell runs it, so it is the same on Windows.
func splitCommand(line string) []string {
	var args []string
	var arg strings.Builder
	quoted, any := false, false
	for _, r := range line {
		switch {
		case r == '"':
			quoted, any = !quoted, true
		case (r == ' ' || r == '\t') && !quoted:
			if any {
				args, any = append(args, arg.String()), false
				arg.Reset()
			}
		default:
			arg.WriteRune(r)
			any = true
		}
	}
	if any {
		args = append(args, arg.String())
	}
	return args
}

// docsReviewRun hands Claude (the claude command, Claude Code) the review prompt with what
// docs-lint found, in this repo, allowed to edit files. The prompt is the same for every repo; the
// standard it applies is docs/charter/writing.md.
func docsReviewRun(args []string) error {
	show := false
	flags("docs-review", args, func(f *flag.FlagSet) { f.BoolVar(&show, "print", false, "print the prompt, don't run Claude") })
	lint := exec.Command(os.Args[0], "docs-lint")
	found, _ := lint.CombinedOutput() // a failing lint is the point: its output goes into the prompt
	prompt := strings.Replace(docsReview, "__LINT__", strings.TrimSpace(string(found)), 1)
	if show {
		fmt.Println(prompt)
		return nil
	}
	if _, err := exec.LookPath("claude"); err != nil {
		return errNoClaude
	}
	// It may edit files and run the checks, nothing else.
	cmd := exec.Command("claude", "-p", prompt, "--permission-mode", "acceptEdits", "--allowedTools",
		"Read,Edit,Write,Glob,Grep,Bash(mise run docs:lint),Bash(mise run charter:check),Bash(mise tasks),Bash(git status:*),Bash(git diff:*),Bash(ls:*)")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr // no stdin: the prompt is the whole input
	fmt.Println("Claude is reviewing docs/ (it prints its report when it is done; watch `git status` for its edits)")
	return cmd.Run()
}
