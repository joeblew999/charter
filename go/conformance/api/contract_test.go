package api

import (
	"testing"

	"github.com/joeblew999/charter/go/humaworkers"
	"github.com/joeblew999/charter/go/specs"
)

// As for the notes API (api/contract_test.go): every constrained request field has an example, and
// every example is valid against its own schema.
func TestExamplesAreThereAndValid(t *testing.T) {
	for _, problem := range specs.Examples(humaworkers.New(config(), Routes(Env{}))) {
		t.Error(problem)
	}
}
