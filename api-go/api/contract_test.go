package api

import (
	"testing"

	"github.com/joeblew999/orpc-api/api-go/humamcp"
	"github.com/joeblew999/orpc-api/api-go/humaworkers"
)

// What a spec does not show and a client would find first. These hold for any contract: they name
// no operation, so they stay as they are when the contract changes.

func TestEveryOperationCanBeItsMCPTool(t *testing.T) {
	for _, problem := range humamcp.Check(humaworkers.New(config(), Routes(Env{}))) {
		t.Error(problem)
	}
}
