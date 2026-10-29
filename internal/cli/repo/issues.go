package repo

import (
	"flag"
	"fmt"
	"strings"

	"github.com/joeblew999/charter/internal/run"
)

// Issues runs charter issues [kind]: the repo's open issues, newest first (what is reported, and what is planned: a plan
// is an issue); with a kind, the body of a new one with the headings of that issue form (charter's
// .github/ISSUE_TEMPLATE/charter-<kind>.yml) and the gh command that files it: gh and the API ignore forms.
func Issues(args []string) {
	fs := flag.NewFlagSet("charter issues", flag.ExitOnError)
	fs.Parse(args)
	r := root()
	if fs.NArg() == 0 {
		check(run.Run(r, nil, "gh", "issue", "list", "--state", "open", "--limit", "100"))
		return
	}
	kind := fs.Arg(0)
	b, err := files.ReadFile("files/.github/ISSUE_TEMPLATE/charter-" + kind + ".yml")
	if strings.ContainsAny(kind, "/.") || err != nil {
		check(fmt.Errorf("issues takes a kind of issue: bug, feature, upstream or plan, not %s", kind))
	}
	fmt.Print(IssueBody(string(b)))
}

// IssueBody is an issue form as the body of a new issue: its headings, each with its hint, and on the first line the
// gh command that files it.
func IssueBody(form string) string {
	lines := strings.Split(form, "\n")
	var title, labels string
	var body []string
	for i, line := range lines {
		text := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "title: "):
			title = strings.Trim(strings.TrimPrefix(line, "title: "), `"`)
		case strings.HasPrefix(line, "labels: "):
			labels = strings.NewReplacer("[", "", "]", "", `"`, "", ", ", ",").Replace(strings.TrimPrefix(line, "labels: "))
		case strings.HasPrefix(text, "label: "):
			body = append(body, "### "+strings.TrimPrefix(text, "label: "), "")
			if i+1 < len(lines) {
				if hint, ok := strings.CutPrefix(strings.TrimSpace(lines[i+1]), "description: "); ok {
					body = append(body, "<!-- "+hint+" -->", "")
				}
			}
			body = append(body, "", "")
		}
	}
	return fmt.Sprintf("<!-- Fill this in, then: gh issue create --title %q --label %s,agent-filed --body-file <this file> -->\n\n%s\n", title+"...", labels, strings.Join(body, "\n"))
}
