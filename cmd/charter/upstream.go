package main

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// charter upstream: the upstream issues the code under this folder works around (every `Upstream:` tag) and their state
// Run by: mise run upstream:status (tasks/repo/tasks.toml)
func init() {
	commands["upstream"] = command{"", "the upstream issues the code under this folder works around (every `Upstream:` tag) and their state", upstream}
	anywhere["upstream"] = true
}

// A tag is a comment line such as:  Upstream: owner/repo#123 (when fixed: what to do then)
var upstreamTag = regexp.MustCompile(`^([^:]+:[0-9]+):.*Upstream: ([\w.-]+/[\w.-]+)#([0-9]+) *(.*)$`)

// upstream finds the tags and asks GitHub for each issue's state. CLOSED means that workaround can
// go (the table in docs/upstream.md says what to do).
func upstream([]string) error {
	// Not where a tag is written about: the docs, the task descriptions, this file and the tool's tests.
	out, err := output(".", "git", "grep", "-n", "-E", `Upstream: [A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#[0-9]+`, "--", ":!*.md", ":!*mise.toml", ":!cmd/charter/upstream.go", ":!cmd/charter/*_test.go")
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		// git grep found nothing: a repo that works around nothing has nothing to remove.
		fmt.Println("0 upstream issues: no Upstream: tag in the code here")
		return nil
	} else if err != nil {
		return errors.New("upstream reads the files git tracks: run it in a git repo")
	}
	places := map[string][]string{}
	for _, line := range strings.Split(out, "\n") {
		if m := upstreamTag.FindStringSubmatch(line); m != nil {
			issue := m[2] + "#" + m[3]
			places[issue] = append(places[issue], fmt.Sprintf("        %s  %s", m[1], m[4]))
		}
	}
	issues := make([]string, 0, len(places))
	for issue := range places {
		issues = append(issues, issue)
	}
	sort.Strings(issues)
	closed := 0
	for _, issue := range issues {
		repo, number, _ := strings.Cut(issue, "#")
		state, title := "UNREACHABLE", ""
		if info, err := output(".", "gh", "api", "repos/"+repo+"/issues/"+number, "--jq", `.state + "\t" + .title`); err == nil {
			state, title, _ = strings.Cut(info, "\t")
			state = strings.ToUpper(state)
		}
		if state == "CLOSED" {
			closed++
		}
		fmt.Printf("%s  %s  %s\n%s\n", state, issue, title, strings.Join(places[issue], "\n"))
	}
	fmt.Printf("\n%d upstream issues, %d closed", len(issues), closed)
	if closed > 0 {
		fmt.Print(": remove those workarounds (each tag says what to do then)")
	}
	fmt.Println()
	return nil
}
