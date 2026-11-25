package registry

import (
	"strings"
	"testing"
)

// TestFormat: a copy is written as the app's gsx fmt writes it.
func TestFormat(t *testing.T) {
	got, err := Format(".", "x.gsx", []byte("package x\n\ncomponent A() {\n<p>{ \"a\" }</p>\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "\n\t<p>{ \"a\" }</p>\n") {
		t.Errorf("Format didn't indent the markup:\n%s", got)
	}
}
