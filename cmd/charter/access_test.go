package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Updating the application keeps what charter doesn't set, such as the MCP login that the dashboard
// turns on (oauth_configuration): a PUT replaces the whole application.
func TestAccessKeepsWhatItDoesNotSet(t *testing.T) {
	t.Setenv("CF_ACCESS_TEAM_DOMAIN", "team.cloudflareaccess.com")
	t.Setenv("CF_ACCESS_GITHUB_IDP_ID", "idp")
	var put map[string]any
	mux := http.NewServeMux()
	answer := func(w http.ResponseWriter, result any) {
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	}
	mux.HandleFunc("GET /accounts/acct/access/service_tokens", func(w http.ResponseWriter, r *http.Request) {
		answer(w, []map[string]any{{"id": "t1", "name": "notes:mac"}, {"id": "t2", "name": "other:pc"}})
	})
	mux.HandleFunc("GET /accounts/acct/access/apps/app1", func(w http.ResponseWriter, r *http.Request) {
		answer(w, map[string]any{"id": "app1", "aud": "aud1", "name": "notes", "domain": "notes.example.dev",
			"oauth_configuration": map[string]any{"enabled": true}, "session_duration": "1h"})
	})
	mux.HandleFunc("PUT /accounts/acct/access/apps/app1", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &put)
		answer(w, map[string]any{"id": "app1", "aud": "aud1", "domain": "notes.example.dev"})
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	before := cloudflareAPI
	cloudflareAPI = server.URL
	defer func() { cloudflareAPI = before }()

	a := accessApp{cf: cloudflare{token: "x", account: "acct"}, host: "notes.example.dev", worker: "notes"}
	if _, err := a.put(&app{ID: "app1", Domain: "notes.example.dev"}, []string{"me@example.com"}); err != nil {
		t.Fatal(err)
	}
	if oauth, _ := put["oauth_configuration"].(map[string]any); oauth["enabled"] != true {
		t.Errorf("oauth_configuration not kept: %v", put["oauth_configuration"])
	}
	if put["session_duration"] != "24h" {
		t.Errorf("session_duration %v: charter's own settings win", put["session_duration"])
	}
	if _, ok := put["aud"]; ok {
		t.Error("a read-only field (aud) was sent back")
	}
	policies, _ := put["policies"].([]any)
	if len(policies) != 2 {
		t.Errorf("policies: %v, want the machines' and the people's", policies)
	}
}
