package project_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joeblew999/charter/project"
)

const sample = `
name = "my-app"
compatibility_date = "2026-09-11"
main = "worker/index.mjs"
modules = ["worker/room.mjs"]
assets = "dist/site"
health = "/healthz"
port = 8913

[[d1]]
binding = "DB"
migrations = "migrations"

[[durable_objects]]
binding = "ROOM"
class = "Room"
migration = "v1"

[[durable_objects]]
binding = "CHAT"
class = "Chat"
migration = "v2"

[[rate_limits]]
binding = "WRITES"
namespace_id = 1001
limit = 60
period = 10

[production]
domain = "app.example.com"
vars = { APP_ENV = "production", APP_DOMAIN = "app.example.com" }

[preview]
vars = { APP_ENV = "preview {preview}", APP_PREVIEW = "1" }
`

func load(t *testing.T, toml string) (*project.Project, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, project.File), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	return project.Load(sub) // found above
}

func TestDeploy(t *testing.T) {
	p, err := load(t, sample)
	if err != nil {
		t.Fatal(err)
	}
	prod, err := p.Deploy("production", "")
	if err != nil || prod.Worker != "my-app" || prod.Database != "my-app" || prod.Domain != "app.example.com" || prod.Vars["APP_ENV"] != "production" {
		t.Errorf("production: %+v, %v", prod, err)
	}
	pr, err := p.Deploy("preview", "pr-7")
	if err != nil || pr.Worker != "my-app" || pr.Preview != "pr-7" || pr.Database != "my-app-preview" || pr.Domain != "" ||
		!maps.Equal(pr.Vars, map[string]string{"APP_ENV": "preview pr-7", "APP_PREVIEW": "1"}) {
		t.Errorf("preview: %+v, %v", pr, err)
	}
	if _, err := p.Deploy("staging", ""); err == nil || !strings.Contains(err.Error(), "no [staging]") {
		t.Errorf("staging without [staging]: %v", err)
	}
	// A database of its own name (as cf deploy named one, <worker>-db), with each environment's suffix.
	named := project.D1{Binding: "DB", Database: "my-app-db"}
	if got := p.DatabaseOf(named, prod); got != "my-app-db" {
		t.Errorf("DatabaseOf production = %s", got)
	}
	if got := p.DatabaseOf(named, pr); got != "my-app-db-preview" {
		t.Errorf("DatabaseOf preview = %s", got)
	}
	if got := p.DatabaseOf(project.D1{Binding: "DB"}, pr); got != "my-app-preview" {
		t.Errorf("DatabaseOf preview, no name = %s", got)
	}
	// Another Worker of the same code (also): its own database, its name in place of the Worker's, as cf named one.
	api := p.As(prod, "my-app-api")
	if got := p.DatabaseOf(named, api); api.Worker != "my-app-api" || api.Domain != "" || got != "my-app-api-db" {
		t.Errorf("As: %+v, DatabaseOf = %s", api, got)
	}
	if _, err := p.Deploy("preview", ""); err == nil {
		t.Error("a preview without a name deployed")
	}
	if tag, classes := p.LastMigration(); tag != "v2" || !slices.Equal(classes, []string{"Chat"}) {
		t.Errorf("LastMigration = %s %v", tag, classes)
	}
	if r, ok := p.RateLimit("WRITES"); !ok || r.Limit != 60 || r.Period != 10 {
		t.Errorf("RateLimit = %+v %v", r, ok)
	}
	t.Setenv("PORT", "")
	if p.LocalPort() != "8913" {
		t.Errorf("LocalPort = %s", p.LocalPort())
	}
	t.Setenv("PORT", "9999")
	if p.LocalPort() != "9999" {
		t.Errorf("LocalPort with PORT = %s", p.LocalPort())
	}
}

func TestProblems(t *testing.T) {
	for _, tc := range []struct{ toml, want string }{
		{`compatibility_date = "2026-09-11"`, "name is required"},
		{`name = "x"` + "\ncompatibility_date = \"2026-09-11\"\nnmae = \"y\"", "unknown keys"},
		{`name = "x"` + "\ncompatibility_date = \"2026-09-11\"\n[[rate_limits]]\nbinding = \"W\"\nnamespace_id = 1\nlimit = 5\nperiod = 30", "period 10 or 60"},
		{`name = "x"` + "\ncompatibility_date = \"2026-09-11\"\n[[durable_objects]]\nbinding = \"R\"\nclass = \"Room\"\nmigration = \"v1\"\n[[durable_objects]]\nbinding = \"H\"\nclass = \"Hub\"", "or none does"},
		{`name = "x"` + "\ncompatibility_date = \"2026-09-11\"\n[[durable_objects]]\nbinding = \"R\"", "a binding and a class"},
	} {
		if _, err := load(t, tc.toml); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: %v, want %q", tc.toml, err, tc.want)
		}
	}
}

// TestBranches: production deploys production, main staging, a release request nothing, any other branch its preview.
func TestBranches(t *testing.T) {
	for branch, want := range map[string]string{"production": "production", "main": "staging", "release/v1.7": "", "fix-x": "preview", "feat/y": "preview"} {
		if got := project.BranchEnvironment(branch); got != want {
			t.Errorf("BranchEnvironment(%q) = %q, want %q", branch, got, want)
		}
	}
	p := &project.Project{Name: "my-app", Subdomain: "you"}
	if got := p.PreviewURL("fix-x"); got != "https://fix-x-my-app.you.workers.dev" {
		t.Errorf("PreviewURL = %s", got)
	}
}

// TestLimits: the contract's x-rate-limit on an operation of the spec is a rate limit beside [[rate_limits]], once
// however many operations name it; one binding said in both, or twice differently, is refused. A binding with no
// namespace_id counts in the one named from the Worker and the binding, the one the deployed Workers count in.
func TestLimits(t *testing.T) {
	spec := `{"paths": {
		"/api/notes": {"post": {"x-rate-limit": {"binding": "WRITE_LIMIT", "limit": 100, "period": 60}}, "get": {}},
		"/api/notes/{id}": {"delete": {"x-rate-limit": {"binding": "WRITE_LIMIT", "limit": 100, "period": 60}}}
	}}`
	limits := func(toml, spec string) ([]project.RateLimit, error) {
		t.Helper()
		p, err := load(t, toml)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p.Root, "openapi.json"), []byte(spec), 0o644); err != nil {
			t.Fatal(err)
		}
		return p.Limits()
	}
	base := "name = \"x\"\ncompatibility_date = \"2026-09-11\"\nspec = \"openapi.json\"\n"
	got, err := limits(base+"[[rate_limits]]\nbinding = \"LOGINS\"\nlimit = 5\nperiod = 60\n", spec)
	want := []project.RateLimit{{Binding: "LOGINS", Limit: 5, Period: 60}, {Binding: "WRITE_LIMIT", Limit: 100, Period: 60}}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Limits = %v, %v; want %v", got, err, want)
	}
	if _, err := limits(base+"[[rate_limits]]\nbinding = \"WRITE_LIMIT\"\nlimit = 5\nperiod = 60\n", spec); err == nil || !strings.Contains(err.Error(), "say it in one") {
		t.Errorf("a binding in both: %v", err)
	}
	if _, err := limits(base, strings.Replace(spec, `"limit": 100, "period": 60}}}`, `"limit": 7, "period": 60}}}`, 1)); err == nil || !strings.Contains(err.Error(), "twice, differently") {
		t.Errorf("declared twice, differently: %v", err)
	}
	// The counter charter-notes-go's WRITE_LIMIT counts in, deployed.
	if n := project.Namespace("charter-notes-go", "WRITE_LIMIT"); n != 390463246 {
		t.Errorf("Namespace = %d, want the deployed 390463246", n)
	}
}

// TestOlderRepos: a charter.toml of an older charter (its repo's settings at the top) is refused naming those keys and
// where they go now, and a charter-ts project (a charter.config.ts, no charter.toml) is pointed to the upgrade page;
// a repo's charter.toml with [github] and jobs, but no Worker, loads with LoadRepo and not with Load.
func TestOlderRepos(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, project.File), []byte("description = \"ours\"\ntopics = [\"go\"]\n"), 0o644)
	if _, err := project.LoadRepo(dir); err == nil || !strings.Contains(err.Error(), "description, topics: an older charter's keys") {
		t.Errorf("an older charter's keys: %v", err)
	}
	os.WriteFile(filepath.Join(dir, project.File), []byte("[github]\ndescription = \"ours\"\n\n[ci]\ncheck = [\"repo\", \"test\"]\n\n[ci.jobs.test]\nabout = \"the tests\"\non = [\"push\"]\nrun = [\"go test ./...\"]\n"), 0o644)
	if p, err := project.LoadRepo(dir); err != nil || p.IsWorker() || p.CI.Jobs["test"].Run[0] != "go test ./..." {
		t.Errorf("a repo's charter.toml: %+v, %v", p, err)
	}
	if _, err := project.Load(dir); err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Errorf("Load of a repo's charter.toml: %v", err)
	}
	os.WriteFile(filepath.Join(dir, project.File), []byte("[ci.jobs.run]\nabout = \"x\"\non = [\"push\"]\nrun = [\"x\"]\n"), 0o644)
	if _, err := project.LoadRepo(dir); err == nil || !strings.Contains(err.Error(), "a step of charter ci's own") {
		t.Errorf("a job named run: %v", err)
	}
	ts := t.TempDir()
	os.WriteFile(filepath.Join(ts, "charter.config.ts"), []byte("export default {}\n"), 0o644)
	if _, err := project.Load(ts); err == nil || !strings.Contains(err.Error(), "charter-ts's") {
		t.Errorf("a charter-ts project: %v", err)
	}
}
