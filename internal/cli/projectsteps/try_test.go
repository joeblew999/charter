package projectsteps

import (
	"testing"

	"github.com/joeblew999/charter/project"
)

// TestTryWithoutAContract: an app with no spec has no standard test (an API's), only its own commands, each given the
// address; one that fails fails the run.
func TestTryWithoutAContract(t *testing.T) {
	p := &project.Project{Root: t.TempDir()}
	// Nothing answers at this address: the standard test would fail here if it ran.
	pl := places{base: "http://127.0.0.1:1"}
	if err := try(p, []string{"sh -c true"}, pl, map[string]string{"BASE": pl.base}); err != nil {
		t.Errorf("an app's own passing test: %v", err)
	}
	if err := try(p, []string{"sh -c false"}, pl, nil); err == nil {
		t.Error("an app's own failing test passed")
	}
	p.Spec = "fern/openapi.json"
	if err := try(p, []string{"sh -c true"}, pl, nil); err == nil {
		t.Error("an API's standard test passed against an address nothing answers at")
	}
}
