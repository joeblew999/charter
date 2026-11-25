package cfdeploy

import (
	"encoding/json"
	"testing"
)

// TestObservability: every upload turns Workers Logs on; traces only at the rate the config gives.
func TestObservability(t *testing.T) {
	if got, _ := json.Marshal(observability(Config{})); string(got) != `{"enabled":true,"head_sampling_rate":1}` {
		t.Errorf("no traces: %s", got)
	}
	if got, _ := json.Marshal(observability(Config{Traces: 0.05})); string(got) != `{"enabled":true,"head_sampling_rate":1,"traces":{"enabled":true,"head_sampling_rate":0.05}}` {
		t.Errorf("traces at 5%%: %s", got)
	}
}
