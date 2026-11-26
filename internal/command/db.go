package command

import (
	"errors"
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

func dbSeed(p *project.Project, args []string) {
	binding, flag := where(p, &args)
	seed := filepath.Join("db", "seed.sql")
	if !exists(filepath.Join(p.Root, seed)) {
		check(errors.New("db/seed.sql: there is none to run"))
	}
	d1(p, flag, "execute", binding, "--file", seed)
}

// dbReset makes the local database again: wrangler's local state for D1 goes, the migrations run, then the seed.
func dbReset(p *project.Project, args []string) {
	if slices.Contains(args, "--deployed") {
		check(errors.New("db:reset is for the local database only"))
	}
	check(os.RemoveAll(filepath.Join(p.Root, ".wrangler", "state", "v3", "d1")))
	binding, flag := where(p, &args)
	d1(p, flag, "migrations", "apply", binding)
	if exists(filepath.Join(p.Root, "db", "seed.sql")) {
		d1(p, flag, "execute", binding, "--file", filepath.Join("db", "seed.sql"))
	}
}

func dbExport(p *project.Project, args []string) {
	binding, flag := where(p, &args)
	if len(args) != 1 {
		check(errors.New("usage: charter run db:export <file.sql> [--deployed]"))
	}
	d1(p, flag, "export", binding, "--output", args[0])
}
