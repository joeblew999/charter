package command

import "testing"

// A go.mod that differs from go mod tidy's only in a // indirect mark is tidy enough: that is what a clone that has
// not been built gives. Anything else is a change.
func TestTidyChanges(t *testing.T) {
	onlyMark := "diff current/go.mod tidy/go.mod\n--- current/go.mod\n+++ tidy/go.mod\n@@ -4,7 +4,6 @@\n-\tgithub.com/gsxhq/gsx v0.5.0\n+\tgithub.com/gsxhq/gsx v0.5.0 // indirect\n"
	if got := tidyChanges(onlyMark); len(got) != 0 {
		t.Errorf("an indirect mark alone is a change: %q", got)
	}
	version := "--- current/go.mod\n+++ tidy/go.mod\n-\tgithub.com/gsxhq/gsx v0.5.0\n+\tgithub.com/gsxhq/gsx v0.6.0\n+\texample.com/new v1.0.0\n"
	if got := tidyChanges(version); len(got) != 3 {
		t.Errorf("a version and a new module are %d changes: %q", len(got), got)
	}
	if got := tidyChanges(""); len(got) != 0 {
		t.Errorf("no diff is a change: %q", got)
	}
}
