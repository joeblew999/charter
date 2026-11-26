package command

import (
	"encoding/json"
	"fmt"
	"github.com/joeblew999/charter/internal/run"
	"io"
	"net/http"
	"strings"

	"errors"
	"github.com/joeblew999/charter/app"
	"os"
	"path/filepath"
	"slices"

	"github.com/joeblew999/charter/internal/project"
	"github.com/tailscale/hujson"
)

// The database tasks of charter run (db:status, db:migrate, …), as Void has them for its databases. Each is wrangler's
// own D1 command on the app's database: the one the binding of wrangler.jsonc's first d1_databases entry names, with
// its migrations in db/migrations. The local database (wrangler dev's) unless --deployed is given.
//
// The schema is db/schema.sql. Its migrations are written from it (charter check --fix; charter dev as it is
// edited), applied locally by charter dev and remotely by charter deploy. The queries are db/queries.sql, written as
// typed Go.

// where is the binding of the app's database and wrangler's flag for which one: --local, or --remote for --deployed
// (taken out of args).
func where(p *project.Project, args *[]string) (binding, flag string) {
	if len(p.Worker.D1) == 0 {
		check(errors.New("wrangler.jsonc has no d1_databases: the app has no database"))
	}
	flag = "--local"
	if i := slices.Index(*args, "--deployed"); i >= 0 {
		*args, flag = slices.Delete(*args, i, i+1), "--remote"
	}
	return p.Worker.D1[0].Binding, flag
}

// d1 runs wrangler d1 <words>: on the local database as it is, on the deployed one with Cloudflare's credentials.
func d1(p *project.Project, flag string, words ...string) {
	words = append([]string{"d1"}, append(words, flag)...)
	if flag == "--remote" {
		check(cloudflare(p, words, nil))
		return
	}
	check(wrangler(p, map[string]string{"CI": "1"}, words...))
}

// db is a task that is wrangler d1 <group> <verb> <binding>.
func db(group, verb string) func(*project.Project, []string) {
	return func(p *project.Project, args []string) {
		binding, flag := where(p, &args)
		d1(p, flag, append([]string{group, verb, binding}, args...)...)
	}
}

func dbExecute(p *project.Project, args []string) {
	binding, flag := where(p, &args)
	if len(args) != 1 {
		check(errors.New(`usage: charter run db:execute "<SQL>" [--deployed]`))
	}
	d1(p, flag, "execute", binding, "--command", args[0])
}

// dbSeed is charter run db:seed, as Void's db seed is: the local database made again (its tables gone, the
// migrations run), then filled by the app's seed. The seed is seed.go, which names what fills it (written with the
// app's own types and functions), or db/seed.sql. Local only: a seed starts from nothing, which a deployed database
// must never be made to.
//
// Before anything is dropped the seed is compiled: one that does not build stops here, with the database as it was.
// And it works beside a running charter dev, whose Worker has the database open: the tables are then dropped and
// made again in place, where otherwise the database's files are removed.
func dbSeed(p *project.Project, args []string) {
	if i := slices.Index(args, "--deployed"); i >= 0 {
		dbSeedDeployed(p, slices.Delete(slices.Clone(args), i, i+1))
		return
	}
	// Nothing but the local database, and no flag that might be read as another: one that is not understood is
	// refused, so that --deployed, --remote or a slip of the keys cannot be taken for a yes.
	if len(args) > 0 {
		check(fmt.Errorf("db:seed takes no arguments (%s): it makes the local database again and fills it. The deployed one is charter run db:seed --deployed, which says what it would delete and how to say yes", strings.Join(args, " ")))
	}
	if exists(filepath.Join(p.Root, "seed.go")) && exists(filepath.Join(p.Root, "db", "seed.sql")) {
		check(errors.New("the app has two seeds, seed.go and db/seed.sql: keep one"))
	}
	if !hasSeed(p) {
		check(errors.New("the app has no seed: seed.go (//go:build seed, with app.Seed in its init, naming what fills the database) or db/seed.sql"))
	}
	if exists(filepath.Join(p.Root, "seed.go")) {
		check(generate(p))
		check(run.Run(p.Root, map[string]string{"GOOS": "js", "GOARCH": "wasm"}, "go", "vet", "-tags", "seed", "."))
	}
	check(resetLocal(p))
	check(seedLocal(p))
}

// dbSeedDeployed is charter run db:seed --deployed: the deployed database emptied and filled by the app's seed, for
// an app whose deployed data is the seed's (a demo, a staging environment). It deletes every row, so it is asked
// for twice. Without the Worker's name it says what the database has and changes nothing; with it (charter run
// db:seed --deployed my-worker) it goes ahead. The tables stay as the migrations made them, so the deployed Worker
// keeps answering throughout.
//
// The seed is the one the local database is filled by, and it still runs here: a Worker on this machine, built
// with the seed, whose database binding is the deployed database (wrangler's remote bindings). The deployed
// Worker never has the seed in it. And what is safe only locally stays out: app.SeedIsDeployed tells the seed, and
// api.SeedPerson keeps no token there.
func dbSeedDeployed(p *project.Project, args []string) {
	if exists(filepath.Join(p.Root, "seed.go")) && exists(filepath.Join(p.Root, "db", "seed.sql")) {
		check(errors.New("the app has two seeds, seed.go and db/seed.sql: keep one"))
	}
	if !hasSeed(p) {
		check(errors.New("the app has no seed: seed.go (//go:build seed, with app.Seed in its init, naming what fills the database) or db/seed.sql"))
	}
	if len(p.Worker.D1) == 0 {
		check(errors.New(project.Wrangler + " has no d1_databases"))
	}
	type rows struct {
		Name string
		N    int64
	}
	// What each database has now, table by table: shown before anything is asked of it.
	has := map[string][]rows{}
	for _, db := range p.Worker.D1 {
		var tables []struct{ Name string }
		check(remote(p, db.Binding, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_cf_%' AND name <> 'd1_migrations' ORDER BY name", &tables))
		if len(tables) == 0 {
			check(fmt.Errorf("the deployed database %s has no tables yet: go tool charter deploy makes them", db.Binding))
		}
		// One row, a column a table: D1 takes few SELECTs joined by UNION.
		var counts []string
		for _, t := range tables {
			counts = append(counts, fmt.Sprintf("(SELECT count(*) FROM \"%s\") AS \"%s\"", t.Name, t.Name))
		}
		var row []map[string]int64
		check(remote(p, db.Binding, "SELECT "+strings.Join(counts, ", "), &row))
		var counted []rows
		for _, t := range tables {
			if len(row) > 0 {
				counted = append(counted, rows{t.Name, row[0][t.Name]})
			}
		}
		has[db.Binding] = counted
		say("the deployed database %s of %s has:", db.Binding, p.Worker.Name)
		for _, t := range counted {
			say("  %-14s %d rows", t.Name, t.N)
		}
	}
	if len(args) != 1 || args[0] != p.Worker.Name {
		check(fmt.Errorf("nothing was changed. To delete every one of those rows and fill the database from the app's seed, name the Worker:\n  go tool charter run db:seed --deployed %s", p.Worker.Name))
	}
	// Before anything is deleted the seed is compiled: one that does not build stops here.
	if exists(filepath.Join(p.Root, "seed.go")) {
		check(generate(p))
		check(run.Run(p.Root, map[string]string{"GOOS": "js", "GOARCH": "wasm"}, "go", "vet", "-tags", "seed", "."))
	}
	for _, db := range p.Worker.D1 {
		// One statement list, checked at its end: the order of the tables does not matter to what refers to what.
		empty := "PRAGMA defer_foreign_keys = true;"
		for _, t := range has[db.Binding] {
			empty += fmt.Sprintf(" DELETE FROM \"%s\";", t.Name)
		}
		// And the ids start again, as they do on a database that is new: the seed's rows are the same rows here.
		var counters []struct{ Name string }
		check(remote(p, db.Binding, "SELECT name FROM sqlite_master WHERE name = 'sqlite_sequence'", &counters))
		if len(counters) > 0 {
			empty += " DELETE FROM sqlite_sequence;"
		}
		check(remote(p, db.Binding, empty, nil))
		say("✓ the deployed database %s is empty", db.Binding)
	}
	if sql := filepath.Join("db", "seed.sql"); exists(filepath.Join(p.Root, sql)) {
		for _, db := range p.Worker.D1 {
			check(cloudflare(p, []string{"d1", "execute", db.Binding, "--remote", "--yes", "--file", sql}, nil))
		}
		say("✓ the deployed database is seeded (db/seed.sql)")
		return
	}
	// wrangler.jsonc again, with each database the deployed one: read by the Worker started here, and removed.
	config, err := remoteConfig(p)
	check(err)
	err = servingWith(p, served{env: map[string]string{seedBuild: "1"}, local: true, config: config, cloudflare: true}, func(base string, _ int) error {
		req, err := http.NewRequest(http.MethodPost, base+app.SeedPath, nil)
		if err != nil {
			return err
		}
		req.Header.Set(app.SeedDeployed, "1")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		said, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("seed.go: %s", strings.TrimSpace(string(said)))
		}
		say("✓ the deployed database is seeded (seed.go)")
		return nil
	})
	os.Remove(filepath.Join(p.Root, config))
	check(err)
}

// remote asks the deployed database of a binding one thing, and reads its rows into into (nil: none are wanted).
func remote(p *project.Project, binding, sql string, into any) error {
	out, err := cloudflareOutput(p, "d1", "execute", binding, "--remote", "--yes", "--json", "--command", sql)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(out))
	}
	return results(out, into)
}

// results reads the rows of wrangler d1 execute --json's last statement into into (nil: none are wanted).
func results(out string, into any) error {
	if into == nil {
		return nil
	}
	var answer []struct{ Results json.RawMessage }
	start := strings.Index(out, "[")
	if start < 0 || json.Unmarshal([]byte(out[start:]), &answer) != nil || len(answer) == 0 {
		return fmt.Errorf("the database did not answer as rows:\n%s", strings.TrimSpace(out))
	}
	return json.Unmarshal(answer[len(answer)-1].Results, into)
}

// remoteConfig writes the Worker's configuration again beside wrangler.jsonc, with every D1 database marked remote
// (https://developers.cloudflare.com/workers/development-testing/#remote-bindings), and gives the file's name.
func remoteConfig(p *project.Project) (string, error) {
	b, err := os.ReadFile(filepath.Join(p.Root, project.Wrangler))
	if err != nil {
		return "", err
	}
	if b, err = hujson.Standardize(b); err != nil {
		return "", err
	}
	var config map[string]any
	if err := json.Unmarshal(b, &config); err != nil {
		return "", err
	}
	databases, _ := config["d1_databases"].([]any)
	for _, db := range databases {
		db, ok := db.(map[string]any)
		if !ok {
			continue
		}
		db["remote"] = true
		// A database wrangler made by its name has no id in the file; a remote binding is found by one.
		if id, _ := db["database_id"].(string); id == "" {
			name, _ := db["database_name"].(string)
			out, err := cloudflareOutput(p, "d1", "info", name, "--json")
			var info struct{ UUID string }
			if start := strings.Index(out, "{"); err != nil || start < 0 || json.Unmarshal([]byte(out[start:]), &info) != nil || info.UUID == "" {
				return "", fmt.Errorf("the deployed database %s was not found (%v):\n%s", name, err, strings.TrimSpace(out))
			}
			db["database_id"] = info.UUID
		}
	}
	if b, err = json.MarshalIndent(config, "", "  "); err != nil {
		return "", err
	}
	const name = "wrangler.charter-seed.json"
	return name, os.WriteFile(filepath.Join(p.Root, name), b, 0o644)
}

// hasSeed reports whether the app has a seed.
func hasSeed(p *project.Project) bool {
	return exists(filepath.Join(p.Root, "seed.go")) || exists(filepath.Join(p.Root, "db", "seed.sql"))
}

// resetLocal makes the local database again from the migrations. With no charter dev running here, its files go.
// Beside one, which has them open, every table is dropped where it stands.
func resetLocal(p *project.Project) error {
	quiet := map[string]string{"WRANGLER_SEND_METRICS": "false", "CI": "1"}
	if _, running := devState(p); !running {
		if err := os.RemoveAll(filepath.Join(p.Root, ".wrangler", "state", "v3", "d1")); err != nil {
			return err
		}
		return migrate(p, quiet, "--local")
	}
	for _, db := range p.Worker.D1 {
		bin, err := npm(p, "wrangler")
		if err != nil {
			return err
		}
		out, err := run.Output(p.Root, quiet, bin, "d1", "execute", db.Binding, "--local", "--json", "--command",
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_cf_%'")
		if err != nil {
			return err
		}
		var answer []struct {
			Results []struct{ Name string }
		}
		if err := json.Unmarshal([]byte(out[strings.Index(out, "["):]), &answer); err != nil || len(answer) == 0 {
			return fmt.Errorf("the local database did not say which tables it has: %v", err)
		}
		drop := "PRAGMA defer_foreign_keys = true;"
		for _, t := range answer[0].Results {
			drop += " DROP TABLE IF EXISTS " + t.Name + ";"
		}
		if err := wrangler(p, quiet, "d1", "execute", db.Binding, "--local", "--command", drop); err != nil {
			return err
		}
	}
	return migrate(p, quiet, "--local")
}

// seedLocal fills the local database, as it is, with the app's seed. SQL is run as it is written. Go runs where
// the app's database code always runs, in a Worker: one is started here on the local database, built with the seed
// in it (the build tag seed), asked to run it, and stopped.
func seedLocal(p *project.Project) error {
	if sql := filepath.Join("db", "seed.sql"); exists(filepath.Join(p.Root, sql)) {
		for _, db := range p.Worker.D1 {
			if err := wrangler(p, map[string]string{"WRANGLER_SEND_METRICS": "false", "CI": "1"}, "d1", "execute", db.Binding, "--local", "--file", sql); err != nil {
				return err
			}
		}
		say("✓ the local database is seeded (db/seed.sql)")
		return nil
	}
	if !exists(filepath.Join(p.Root, "seed.go")) {
		return nil
	}
	return servingWith(p, served{env: map[string]string{seedBuild: "1"}, local: true}, func(base string, _ int) error {
		res, err := http.Post(base+app.SeedPath, "text/plain", nil)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		said, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("seed.go: %s", strings.TrimSpace(string(said)))
		}
		say("✓ the local database is seeded (seed.go)")
		return nil
	})
}

// dbReset is db:seed for an app that may have no seed: the local database made again from the migrations, and
// filled when there is a seed.
func dbReset(p *project.Project, args []string) {
	if len(args) > 0 {
		check(fmt.Errorf("db:reset takes no arguments (%s): it makes the local database again, and only that one", strings.Join(args, " ")))
	}
	check(resetLocal(p))
	if hasSeed(p) {
		check(seedLocal(p))
	}
}

func dbExport(p *project.Project, args []string) {
	binding, flag := where(p, &args)
	if len(args) != 1 {
		check(errors.New("usage: charter run db:export <file.sql> [--deployed]"))
	}
	d1(p, flag, "export", binding, "--output", args[0])
}
