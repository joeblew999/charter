package cfdeploy_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/joeblew999/charter/go/lib/internal/cfdeploy"
)

// fakeBuilds is the slice of the Cloudflare API ConfigureBuilds uses, for account "acc", Worker "app" (tag "t1").
type fakeBuilds struct {
	mu       sync.Mutex
	tokens   []map[string]string
	conns    int
	config   map[string]any            // POST /builds/workers body
	triggers map[string]map[string]any // uuid → trigger
	vars     map[string]map[string]any // trigger uuid → variables
	n        int
}

func newFakeBuilds(t *testing.T) (*fakeBuilds, *cfdeploy.Client) {
	f := &fakeBuilds{triggers: map[string]map[string]any{}, vars: map[string]map[string]any{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		reply := func(v any) {
			json.NewEncoder(w).Encode(map[string]any{"success": true, "errors": []any{}, "result": v})
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		p := strings.TrimPrefix(r.URL.Path, "/accounts/acc")
		switch {
		case r.Method == "GET" && p == "/workers/scripts":
			reply([]map[string]string{{"id": "app", "tag": "t1"}})
		case r.Method == "GET" && p == "/builds/tokens":
			reply(f.tokens)
		case r.Method == "PUT" && p == "/builds/repos/connections":
			if body["repo_id"] != "42" || body["provider_account_id"] != "7" || body["provider_type"] != "github" {
				t.Errorf("connection body %v", body)
			}
			f.conns++
			reply(map[string]string{"repo_connection_uuid": "conn-1"})
		case r.Method == "GET" && p == "/builds/workers/t1/triggers":
			var out []map[string]any
			for _, tr := range f.triggers {
				out = append(out, tr)
			}
			reply(out)
		case r.Method == "POST" && p == "/builds/workers":
			f.config = body
			f.n++
			id := "prod-" + string(rune('0'+f.n))
			f.triggers[id] = map[string]any{"trigger_uuid": id, "trigger_name": "t1", "branch_includes": []string{"main"}}
			reply(nil)
		case r.Method == "POST" && p == "/builds/triggers":
			f.n++
			id := "trig-" + string(rune('0'+f.n))
			body["trigger_uuid"] = id
			f.triggers[id] = body
			reply(body)
		case r.Method == "PATCH" && strings.HasPrefix(p, "/builds/triggers/") && strings.HasSuffix(p, "/environment_variables"):
			f.vars[strings.Split(p, "/")[3]] = body
			reply(nil)
		case r.Method == "PATCH" && strings.HasPrefix(p, "/builds/triggers/"):
			id := strings.TrimPrefix(p, "/builds/triggers/")
			for k, v := range body {
				f.triggers[id][k] = v
			}
			reply(nil)
		default:
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []any{map[string]any{"code": 404, "message": r.Method + " " + p}}})
		}
	}))
	t.Cleanup(srv.Close)
	return f, &cfdeploy.Client{Token: "tok", AccountID: "acc", BaseURL: srv.URL}
}

func buildsConfig() cfdeploy.BuildsConfig {
	return cfdeploy.BuildsConfig{
		Worker: "app", Owner: "o", Repo: "r", OwnerID: "7", RepoID: "42",
		BuildToken: "app build token", FallbackBuildToken: "other build token",
		BuildCommand: "ci/workers-builds.sh build", DeployCommand: "ci/workers-builds.sh deploy",
		Vars: map[string]string{"CLOUDFLARE_ACCOUNT_ID": "acc"}, PathExcludes: []string{"*.md"},
	}
}

func TestConfigureBuilds(t *testing.T) {
	f, c := newFakeBuilds(t)
	ctx := context.Background()

	if _, err := c.ConfigureBuilds(ctx, buildsConfig()); err == nil || !strings.Contains(err.Error(), "no build token") {
		t.Fatalf("no tokens: %v", err)
	}
	f.tokens = []map[string]string{{"build_token_uuid": "tok-other", "build_token_name": "other build token"}}

	// From nothing: a build configuration (echo-only production) and the preview trigger, on the fallback token.
	id, err := c.ConfigureBuilds(ctx, buildsConfig())
	if err != nil {
		t.Fatal(err)
	}
	if f.config == nil {
		t.Fatal("no build configuration created")
	}
	for tid, tr := range f.triggers { // the main trigger deploys the main preview (staging) with the same commands
		if tid != id && (tr["deploy_command"] != "ci/workers-builds.sh deploy" || f.vars[tid] == nil) {
			t.Errorf("main trigger %s: %v vars %v", tid, tr, f.vars[tid])
		}
	}
	tr := f.triggers[id]
	if tr["trigger_name"] != "app branch previews" || tr["build_token_uuid"] != "tok-other" || tr["repo_connection_uuid"] != "conn-1" {
		t.Errorf("trigger %v", tr)
	}
	if ex := tr["branch_excludes"].([]any); len(ex) != 1 || ex[0] != "main" {
		t.Errorf("branch_excludes %v", ex)
	}
	if v := f.vars[id]["CLOUDFLARE_ACCOUNT_ID"].(map[string]any); v["value"] != "acc" {
		t.Errorf("vars %v", f.vars[id])
	}

	// Again, with the real token now: same trigger updated, nothing duplicated.
	f.tokens = append(f.tokens, map[string]string{"build_token_uuid": "tok-app", "build_token_name": "app build token"})
	cfg := buildsConfig()
	cfg.DeployCommand = "ci/workers-builds.sh deploy --v2"
	id2, err := c.ConfigureBuilds(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != id || len(f.triggers) != 2 || f.triggers[id]["build_token_uuid"] != "tok-app" || f.triggers[id]["deploy_command"] != cfg.DeployCommand {
		t.Errorf("rerun: id %s→%s, triggers %v", id, id2, f.triggers)
	}
	if f.conns != 2 {
		t.Errorf("connection upserts %d, want 2", f.conns)
	}

	if _, err := c.ConfigureBuilds(ctx, cfdeploy.BuildsConfig{Worker: "app"}); err == nil {
		t.Error("incomplete config accepted")
	}
}
