package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/joeblew999/charter/internal/project"
)

// A deployed app is two Workers from one wrangler.jsonc: the Go Worker has every binding and no address, the
// Worker in front has the address, the browser's files and nothing of the app's.
func TestOneConfigurationBecomesTwoWorkers(t *testing.T) {
	dir := t.TempDir()
	written := `{
  // An app.
  "name": "notes",
  "main": "build/worker.mjs",
  "compatibility_date": "2026-09-25",
  "cache": { "enabled": true },
  "assets": { "directory": "dist" },
  "d1_databases": [{ "binding": "DB", "database_id": "1" }],
  "durable_objects": { "bindings": [{ "name": "HUB", "class_name": "Hub" }] },
  "migrations": [{ "tag": "v1", "new_sqlite_classes": ["Hub"] }],
  "triggers": { "crons": ["0 6 * * *"] },
  "env": { "staging": { "assets": { "directory": "dist" }, "vars": { "A": "b" } } }
}`
	if err := os.WriteFile(filepath.Join(dir, project.Wrangler), []byte(written), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &project.Project{Root: dir}
	p.Worker.Name = "notes"
	read := func(name string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var c map[string]any
		if err := json.Unmarshal(b, &c); err != nil {
			t.Fatal(err)
		}
		return c
	}

	if err := writeGo(p); err != nil {
		t.Fatal(err)
	}
	goes := read(goConfig)
	if goes["name"] != "notes-go" || goes["workers_dev"] != false {
		t.Errorf("the Go Worker: name %v, workers_dev %v", goes["name"], goes["workers_dev"])
	}
	for _, key := range []string{"assets", "cache"} {
		if goes[key] != nil {
			t.Errorf("the Go Worker has %s, which is the Worker in front's", key)
		}
	}
	for _, key := range []string{"d1_databases", "durable_objects", "migrations", "triggers"} {
		if goes[key] == nil {
			t.Errorf("the Go Worker has no %s", key)
		}
	}
	if staging := goes["env"].(map[string]any)["staging"].(map[string]any); staging["assets"] != nil || staging["vars"] == nil {
		t.Errorf("the Go Worker's staging: %v", staging)
	}

	if err := writeFront(p, "", nil); err != nil {
		t.Fatal(err)
	}
	front := read(frontConfig)
	if front["name"] != "notes" || front["main"] != "build/front-worker.mjs" || front["assets"] == nil || front["cache"] == nil {
		t.Errorf("the Worker in front: %v", front)
	}
	for _, key := range []string{"d1_databases", "durable_objects", "migrations", "env", "build"} {
		if front[key] != nil {
			t.Errorf("the Worker in front has %s", key)
		}
	}
	if service := front["services"].([]any)[0].(map[string]any); service["service"] != "notes-go" {
		t.Errorf("the Worker in front hands requests to %v", service["service"])
	}
	if crons := front["triggers"].(map[string]any)["crons"].([]any); len(crons) != 0 {
		t.Errorf("the Worker in front runs at %v", crons)
	}

	// The Worker of the app's name gives up the classes it had as the whole app, after the migrations that made them.
	if err := writeFront(p, "staging", []string{"Hub"}); err != nil {
		t.Fatal(err)
	}
	front = read(frontConfig)
	if front["name"] != "notes-staging" || front["services"].([]any)[0].(map[string]any)["service"] != "notes-go-staging" {
		t.Errorf("staging's Worker in front: %v", front)
	}
	migrations := front["migrations"].([]any)
	if last := migrations[len(migrations)-1].(map[string]any); len(migrations) != 2 || last["tag"] != "charter-front" || last["deleted_classes"].([]any)[0] != "Hub" {
		t.Errorf("its migrations: %v", migrations)
	}
}
