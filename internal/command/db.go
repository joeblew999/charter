package command

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"errors"
	"github.com/joeblew999/charter/app"
	"os"
	"path/filepath"
	"slices"

	"github.com/joeblew999/charter/internal/project"
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

// dbSeed is charter run db:seed, as Void's db seed is: the local database made again (its state gone, the
// migrations run), then filled by the app's seed. The seed is db/seed.go, Go with the app's own typed queries, or
// db/seed.sql. Local only: a seed starts from nothing, which a deployed database must never be made to.
func dbSeed(p *project.Project, args []string) {
	if slices.Contains(args, "--deployed") {
		check(errors.New("db:seed makes the database again before it fills it: it is for the local database only (a deployed one keeps its rows; charter run db:execute --deployed runs a statement there)"))
	}
	if !hasSeed(p) {
		check(errors.New("the app has no seed: db/seed.go (Go, with the app's queries; //go:build seed, and app.Seed in its init) or db/seed.sql"))
	}
	check(os.RemoveAll(filepath.Join(p.Root, ".wrangler", "state", "v3", "d1")))
	check(migrate(p, map[string]string{"WRANGLER_SEND_METRICS": "false"}, "--local"))
	check(seedLocal(p))
}

// hasSeed reports whether the app has a seed.
func hasSeed(p *project.Project) bool {
	return exists(filepath.Join(p.Root, "db", "seed.go")) || exists(filepath.Join(p.Root, "db", "seed.sql"))
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
	}
	if !exists(filepath.Join(p.Root, "db", "seed.go")) {
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
			return fmt.Errorf("db/seed.go: %s", strings.TrimSpace(string(said)))
		}
		say("✓ the local database is seeded (db/seed.go)")
		return nil
	})
}

// dbReset is db:seed for an app that may have no seed: the local database made again from the migrations, and
// filled when there is a seed.
func dbReset(p *project.Project, args []string) {
	if slices.Contains(args, "--deployed") {
		check(errors.New("db:reset is for the local database only"))
	}
	check(os.RemoveAll(filepath.Join(p.Root, ".wrangler", "state", "v3", "d1")))
	check(migrate(p, map[string]string{"WRANGLER_SEND_METRICS": "false"}, "--local"))
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
