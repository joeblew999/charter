package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// charter migrate:local: apply migrations/*.sql to a running dev server's local D1, each once (the port: API_PORT)
// Run by: mise run migrate:local (tasks/repo/tasks.toml); mise run test:workerd (the examples' mise.toml)
func init() {
	commands["migrate:local"] = command{"[-port <port>]",
		"apply migrations/*.sql to a running dev server's local D1, each once (the port: API_PORT)", migrateLocal}
}

// migrateLocal applies each migration once through the dev server's D1 explorer API (cf can't
// migrate the local database), remembering them in a table of its own.
func migrateLocal(args []string) error {
	var port string
	worker, err := workerFlags("migrate-local", args, &port)
	if err != nil {
		return err
	}
	if port == "" {
		if port, err = env("API_PORT"); err != nil {
			return err
		}
	}
	base := "http://localhost:" + port + "/cdn-cgi/local/explorer/api/d1/database/DB-" + worker + "/raw"
	sql := func(statement string) (first any, err error) {
		body, _ := json.Marshal(map[string]string{"sql": statement})
		res, err := http.Post(base, "application/json", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("no %s dev server on port %s", worker, port)
		}
		defer res.Body.Close()
		var reply struct {
			Success bool
			Errors  []any
			Result  []struct{ Results struct{ Rows [][]any } }
		}
		if err := json.NewDecoder(res.Body).Decode(&reply); err != nil || !reply.Success {
			return nil, fmt.Errorf("local D1 DB-%s refused (HTTP %d %v): %s", worker, res.StatusCode, reply.Errors, statement)
		}
		if n := len(reply.Result); n > 0 && len(reply.Result[n-1].Results.Rows) > 0 && len(reply.Result[n-1].Results.Rows[0]) > 0 {
			first = reply.Result[n-1].Results.Rows[0][0]
		}
		return first, nil
	}
	if _, err := sql("CREATE TABLE IF NOT EXISTS _local_migrations (name TEXT PRIMARY KEY, applied_at TEXT DEFAULT CURRENT_TIMESTAMP)"); err != nil {
		return err
	}
	files, err := migrations()
	if err != nil {
		return err
	}
	applied := 0
	for _, file := range files {
		name := filepath.Base(file)
		count, err := sql("SELECT count(*) FROM _local_migrations WHERE name = '" + name + "'")
		if err != nil {
			return err
		}
		if n, _ := count.(float64); n > 0 {
			continue
		}
		statement, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := sql(string(statement)); err != nil {
			return err
		}
		if _, err := sql("INSERT INTO _local_migrations (name) VALUES ('" + name + "')"); err != nil {
			return err
		}
		fmt.Println("applied", name)
		applied++
	}
	fmt.Printf("local DB-%s: %d migration(s) applied\n", worker, applied)
	return nil
}
