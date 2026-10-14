package main

import (
	"encoding/json"
	"fmt"
)

// charter migrate: REMOTE: apply pending migrations/ to the Worker's D1 database (<worker>-db; the Worker: API_URL's)
// Run by: mise run migrate (tasks/repo/tasks.toml)
func init() {
	commands["migrate"] = command{"[-worker <name>]",
		"REMOTE: apply pending migrations/ to the Worker's D1 database (<worker>-db; the Worker: API_URL's)", migrate}
}

// migrate finds the database's UUID (cf d1 takes UUIDs only, and lists one page unless asked for
// more) and applies what is pending.
func migrate(args []string) error {
	worker, err := workerFlags("migrate", args, nil)
	if err != nil {
		return err
	}
	database := worker + "-db"
	if _, err := migrations(); err != nil {
		return err
	}
	out, err := output(".", npmBin("cf"), "d1", "list", "--name", database, "--per-page", "100")
	if err != nil {
		return err
	}
	var databases []struct{ Name, UUID string }
	if err := json.Unmarshal([]byte(out), &databases); err != nil {
		return fmt.Errorf("cf d1 list: %w", err)
	}
	for _, d := range databases {
		if d.Name == database {
			return sh(".", npmBin("cf"), "d1", "migrations", "apply", d.UUID, "--dir", "migrations")
		}
	}
	return fmt.Errorf("no D1 database %s yet: deploy its Worker first", database)
}
