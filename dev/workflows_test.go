package main

import (
	"regexp"
	"strings"
	"testing"
)

// The rules the templates keep: a clear prefix, pinned actions and runners, and no logic in YAML
// (a step that does work is one mise task, so it runs the same locally).
func TestWorkflowTemplates(t *testing.T) {
	templates, err := workflowTemplates(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) == 0 {
		t.Fatal("no templates")
	}
	prefix := regexp.MustCompile(`^(api|sdk|dev)-[a-z]+\.yml$`)
	pinned := regexp.MustCompile(`^[\w.-]+/[\w.-]+@v\d+\.\d+\.\d+$`)
	for name, template := range templates {
		if !prefix.MatchString(name) {
			t.Errorf("%s: the name must start with api-, sdk- or dev-", name)
		}
		if want := "name: " + strings.TrimSuffix(name, ".yml") + "\n"; !strings.Contains(string(template), "\n"+want) {
			t.Errorf("%s: no %q", name, want)
		}
		for number, line := range strings.Split(string(template), "\n") {
			line = strings.TrimSpace(line)
			at := func(format string, a ...any) { t.Errorf("%s:%d: "+format, append([]any{name, number + 1}, a...)...) }
			if run, ok := strings.CutPrefix(line, "- run: "); ok && !strings.HasPrefix(run, "mise run ") {
				at("a step must be `mise run <task>`, not %q", run)
			}
			if line == "run: |" || line == "- run: |" || line == "shell:" {
				at("no shell blocks: add a mise task")
			}
			if uses, ok := strings.CutPrefix(line, "- uses: "); ok && !pinned.MatchString(uses) {
				at("pin the action to an exact version (owner/repo@vX.Y.Z), not %q", uses)
			}
			if runner, ok := strings.CutPrefix(line, "runs-on: "); ok && strings.Contains(runner, "latest") {
				at("pin the runner (ubuntu-24.04), not %q", runner)
			}
		}
	}
}

func TestWorkflowPrefixes(t *testing.T) {
	templates, err := workflowTemplates([]string{"api", "sdk"})
	if err != nil {
		t.Fatal(err)
	}
	for name := range templates {
		if strings.HasPrefix(name, "dev-") {
			t.Errorf("%s is not an api or sdk workflow", name)
		}
	}
	if _, err := workflowTemplates([]string{"nope"}); err == nil {
		t.Error("an unknown prefix must fail")
	}
}

func TestVersionTags(t *testing.T) {
	for tag, ok := range map[string]bool{
		"v1.2.3": true, "v0.0.0-ci-test": true, "v1.2.3-rc.1": true,
		"1.2.3": false, "v1.2": false, "v01.2.3": false, "dev/v1.2.3": false, "v1.2.3+build": false, "": false,
	} {
		if version.MatchString(tag) != ok {
			t.Errorf("%q: version tag = %v, want %v", tag, !ok, ok)
		}
	}
}
