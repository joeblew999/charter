package cftail

import (
	"encoding/json"
	"testing"
)

// TestFilter: a filter as the API takes it (wrangler's form), and what it refuses.
func TestFilter(t *testing.T) {
	if got, _ := json.Marshal((Filter{}).api()); string(got) != `[]` {
		t.Errorf("no filter: %s", got)
	}
	f := Filter{Status: []string{"error", "canceled"}, Methods: []string{"POST"}, Header: "X-Topic: lobby", Search: "boom", IPs: []string{"self"}, Sampling: 0.25, Version: "v1"}
	want := `[{"sampling_rate":0.25},{"outcome":["exception","exceededCpu","exceededMemory","unknown","canceled"]},{"method":["POST"]},` +
		`{"header":{"key":"X-Topic","query":"lobby"}},{"client_ip":["self"]},{"query":"boom"},{"scriptVersion":"v1"}]`
	if got, _ := json.Marshal(f.api()); string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if got, _ := json.Marshal((Filter{Header: "X-Topic"}).api()); string(got) != `[{"header":{"key":"X-Topic"}}]` {
		t.Errorf("a header by name alone: %s", got)
	}
	if err := f.Check(); err != nil {
		t.Errorf("a good filter: %v", err)
	}
	for _, bad := range []Filter{{Status: []string{"failed"}}, {Sampling: 1}, {Sampling: -0.1}} {
		if bad.Check() == nil {
			t.Errorf("%+v passed the check", bad)
		}
	}
}
