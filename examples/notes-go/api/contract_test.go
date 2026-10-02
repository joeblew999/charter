package api

import (
	"testing"

	"github.com/joeblew999/charter/go/humamcp"
	"github.com/joeblew999/charter/go/humaworkers"
	"github.com/joeblew999/charter/go/specfile"
)

// What a spec does not show and a client would find first. These hold for any contract: they name
// no operation, so they stay as they are when the contract changes.

func TestEveryOperationCanBeItsMCPTool(t *testing.T) {
	for _, problem := range humamcp.Check(humaworkers.New(config(), Routes(Env{}))) {
		t.Error(problem)
	}
}

// Fern shows an example of every request in each SDK's README and reference, and makes up what the
// contract doesn't say: a made-up value fails a pattern or a bound, and whoever copies it gets a 422.
func TestExamplesAreThereAndValid(t *testing.T) {
	for _, problem := range specfile.Examples(humaworkers.New(config(), Routes(Env{}))) {
		t.Error(problem)
	}
}
