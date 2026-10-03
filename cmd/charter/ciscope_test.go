package main

import (
	"slices"
	"testing"
)

func TestCIScope(t *testing.T) {
	t.Chdir("../..")
	examples := []string{"examples/notes-go", "examples/notes-ts", "examples/start-go", "examples/start-ts"}
	for _, c := range []struct {
		name                     string
		changed                  []string
		all                      bool
		tool, library, libraryTS bool
		examples                 []string
	}{
		{name: "a push runs everything", all: true, tool: true, library: true, libraryTS: true, examples: examples},
		{name: "docs only", changed: []string{"docs/README.md"}, examples: []string{}},
		{name: "the tool", changed: []string{"cmd/charter/specdiff.go"}, tool: true, examples: []string{}},
		{name: "the Go library", changed: []string{"go/auth/auth.go"}, library: true, examples: []string{"examples/notes-go", "examples/start-go"}},
		{name: "the TypeScript library", changed: []string{"ts/src/auth.ts"}, libraryTS: true, examples: []string{"examples/notes-ts", "examples/start-ts"}},
		{name: "one example", changed: []string{"examples/notes-ts/src/index.ts"}, examples: []string{"examples/notes-ts"}},
		{name: "the shared tasks", changed: []string{"tasks/shared/sdk.toml"}, tool: true, library: true, libraryTS: true, examples: examples},
		{name: "a workflow", changed: []string{".github/workflows/check.yml"}, tool: true, library: true, libraryTS: true, examples: examples},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := scopeOf(c.changed, examples, c.all)
			if got.tool != c.tool || got.library != c.library || got.libraryTS != c.libraryTS || !slices.Equal(got.examples, c.examples) {
				t.Errorf("got %+v, want tool %v library %v library-ts %v examples %v", got, c.tool, c.library, c.libraryTS, c.examples)
			}
		})
	}
}
