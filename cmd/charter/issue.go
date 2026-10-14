package main

import (
	"errors"
	"fmt"
	"strings"
)

// charter issue: print an issue body with the headings of that issue form, for gh issue create --body-file
// Run by: mise run issue (tasks/repo/tasks.toml)
func init() {
	commands["issue"] = command{"<bug|feature|upstream|plan>", "print an issue body with the headings of that issue form, for gh issue create --body-file", issue}
	anywhere["issue"] = true
}

func issue(args []string) error {
	if len(args) != 1 {
		return errors.New("issue needs the kind: bug, feature, upstream or plan")
	}
	name := "ISSUE_TEMPLATE/charter-" + args[0] + ".yml"
	form, ok := formFiles()[name]
	if !ok || strings.ContainsAny(args[0], "/.") {
		return errors.New("no such issue form: bug, feature, upstream or plan")
	}
	title, formLabels := "", ""
	var body []string
	lines := strings.Split(string(form), "\n")
	for i, line := range lines {
		text := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "title: "):
			title = strings.Trim(strings.TrimPrefix(line, "title: "), `"`)
		case strings.HasPrefix(line, "labels: "):
			formLabels = strings.NewReplacer("[", "", "]", "", `"`, "", ", ", ",").Replace(strings.TrimPrefix(line, "labels: "))
		case strings.HasPrefix(text, "label: "):
			body = append(body, "### "+strings.TrimPrefix(text, "label: "), "")
			if i+1 < len(lines) {
				if hint := strings.TrimSpace(lines[i+1]); strings.HasPrefix(hint, "description: ") {
					body = append(body, "<!-- "+strings.TrimPrefix(hint, "description: ")+" -->", "")
				}
			}
			body = append(body, "", "")
		}
	}
	fmt.Printf("<!-- Fill this in, then: gh issue create --title %q --label %s,agent-filed --body-file <this file> -->\n\n", title+"...", formLabels)
	fmt.Print(strings.Join(body, "\n"))
	return nil
}
