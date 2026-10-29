package repo

import (
	"flag"
	"fmt"
	"maps"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

// tag is a line git grep found with an upstream issue's tag: Upstream: owner/repo#123 (when fixed: what to do then).
var tag = regexp.MustCompile(`^([^:]+:[0-9]+):.*Upstream: ([\w.-]+/[\w.-]+)#([0-9]+) *(.*)$`)

// Upstream runs charter upstream: the upstream issues the code works around, and their state. CLOSED means that
// workaround can go: the tag says what to do then.
func Upstream(args []string) {
	fs := flag.NewFlagSet("charter upstream", flag.ExitOnError)
	fs.Parse(args)
	r := root()
	// Not where a tag is written about: the docs, and the code that reads tags.
	grep := exec.Command("git", "grep", "-n", "-E", "Upstream: [A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#[0-9]+", "--", ":!*.md", ":!internal/cli/repo/upstream.go")
	grep.Dir = r
	out, err := grep.Output()
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
		fmt.Println("0 upstream issues: no Upstream: tag in the code here")
		return
	}
	check(err)
	places := map[string][]string{}
	for _, line := range strings.Split(string(out), "\n") {
		m := tag.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		issue := m[2] + "#" + m[3]
		places[issue] = append(places[issue], fmt.Sprintf("        %s  %s", m[1], m[4]))
	}
	closed := 0
	for _, issue := range slices.Sorted(maps.Keys(places)) {
		repo, number, _ := strings.Cut(issue, "#")
		state, title := "UNREACHABLE", ""
		if out, err := gh(r, "api", "repos/"+repo+"/issues/"+number, "--jq", `.state + "\t" + .title`); err == nil {
			state, title, _ = strings.Cut(strings.TrimSpace(out), "\t")
			state = strings.ToUpper(state)
		}
		if state == "CLOSED" {
			closed++
		}
		fmt.Printf("%s  %s  %s\n%s\n", state, issue, title, strings.Join(places[issue], "\n"))
	}
	fmt.Printf("\n%d upstream issues, %d closed", len(places), closed)
	if closed > 0 {
		fmt.Print(": remove those workarounds (each tag says what to do then)")
	}
	fmt.Println()
}
