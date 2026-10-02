package main

import (
	"errors"
	"fmt"
	"strings"
)

// The issue forms are for the web. gh and the API ignore forms, so an agent would file a body with
// none of the fields triage needs: `charter issue <kind>` prints the form's headings as a body to
// fill in, and the gh command that files it with the form's labels.

func init() {
	commands["issue"] = command{"<bug|feature|upstream>", "print an issue body with the headings of that issue form, for gh issue create --body-file", issue}
	commands["labels"] = command{"", "REMOTE: create or update this repo's GitHub labels from .github/labels.tsv", labels}
	anywhere["issue"], anywhere["labels"] = true, true
}

func issue(args []string) error {
	if len(args) != 1 {
		return errors.New("issue needs the kind: bug, feature or upstream")
	}
	form, err := collaborationFiles.ReadFile("github/ISSUE_TEMPLATE/" + args[0] + ".yml")
	if err != nil {
		return errors.New("no such issue form: bug, feature or upstream")
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

func labels([]string) error {
	rows, err := collaborationFiles.ReadFile("github/labels.tsv")
	if err != nil {
		return err
	}
	for _, row := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		field := strings.Split(row, "\t")
		if len(field) != 3 {
			continue
		}
		if err := quiet(".", nil, "gh", "label", "create", field[0], "--color", field[1], "--description", field[2], "--force"); err != nil {
			return err
		}
		fmt.Println("label", field[0])
	}
	return nil
}
