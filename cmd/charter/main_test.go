package main

import (
	"slices"
	"testing"

	"github.com/joeblew999/charter/internal/cli/repo"
)

// TestCommandsAreSteps: every command docs/charter/README.md and charter help name is one charter runs, and every
// step charter runs is named there, or among the others charter help lists.
func TestCommandsAreSteps(t *testing.T) {
	named := slices.Clone(repo.Extra)
	for _, c := range repo.Commands {
		named = append(named, c.Name)
		if steps[c.Name] == nil {
			t.Errorf("repo.Commands names %s, which charter does not run", c.Name)
		}
	}
	for name := range steps {
		if !slices.Contains(named, name) {
			t.Errorf("charter runs %s, which neither repo.Commands nor repo.Extra names", name)
		}
	}
	for _, name := range repo.Extra {
		if steps[name] == nil {
			t.Errorf("repo.Extra names %s, which charter does not run", name)
		}
	}
}
