package command

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/dbcode"
	"github.com/joeblew999/charter/internal/docsgen"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"path/filepath"
	"strings"
)

const library = "github.com/joeblew999/charter"

// devBuilt is set for the wrangler dev that charter dev runs: the build is already there.
const devBuilt = "CHARTER_DEV_BUILT"

// Build runs charter build: everything wrangler needs to run or deploy the Worker. wrangler.jsonc names it as its
// build command, so wrangler dev and wrangler deploy run it themselves.
//
//	generate   the messages (an charter.toml's [intl]), the components' Go (a gsx.toml), and the browser's files into dist/
//	           (Vite, a vite.config.ts)
//	go         the Wasm into build/, with workers-go's JavaScript and charter's glue, which keeps Go runtimes
//	           between requests
//
// -only generate stops before the Wasm: what go test and go vet need (the generated Go, and Vite's manifest, which
// main.go embeds).
func Build(args []string) {
	fs := flag.NewFlagSet("charter build", flag.ExitOnError)
	only := fs.String("only", "", `"generate": stop once the Go that go test and go vet need is written; "messages": the languages alone, as charter dev compiles them on a change to a catalog; "db": the schema alone, "docs": the docs alone, likewise`)
	dev := fs.Bool("dev", false, "the Wasm only: what charter dev runs on each save, after gsx has regenerated")
	limit := fs.Int("limit", 0, "fail when build/app.wasm is more than this many bytes gzipped (default: charter.toml's [build] limit)")
	fs.Parse(args)
	// Under charter dev the build is done before wrangler starts: wrangler's own call of its build command has
	// nothing left to do, which is what lets it start in a second.
	if os.Getenv(devBuilt) != "" {
		return
	}
	p := load()
	if *limit == 0 {
		*limit = p.Build.Limit
	}
	if *dev {
		check(wasm(p, 0))
		return
	}
	if *only == "docs" {
		// As charter dev does on a change to docs/*.md.
		_, err := docsgen.Write(p.Root)
		check(err)
		return
	}
	if *only == "db" {
		// As charter dev does on a change to db/schema.sql or db/queries.sql: the migration for the change, the
		// queries' Go, and the local database brought to the schema.
		check(pushSchema(p))
		return
	}
	if *only == "messages" {
		if hasIntl(p) {
			intl.Main([]string{"gen"})
		}
		return
	}
	check(generate(p))
	if *only == "generate" {
		return
	}
	check(wasm(p, *limit))
}

// generate writes what the app's own Go needs before it compiles: the messages, the components' Go, Vite's output.
func generate(p *project.Project) error {
	// The docs (docs/*.md) as Go, before anything compiles it.
	if _, err := docsgen.Write(p.Root); err != nil {
		return err
	}
	if hasIntl(p) {
		intl.Main([]string{"gen"})
		if exists(filepath.Join(p.Root, "web")) {
			intl.Main([]string{"assets", "-browser", filepath.Join(".charter", "intl", "browser")})
		}
	}
	if hasGSX(p) {
		if err := run.Run(p.Root, nil, "go", "tool", "gsx", "generate"); err != nil {
			return err
		}
	}
	// The browser's files, with the manifest the Go embeds: before anything compiles the Go.
	if hasVite(p) {
		vite, err := npm(p, "vite")
		if err != nil {
			return err
		}
		return run.Run(p.Root, nil, vite, "build")
	}
	return nil
}

// wasm builds the Worker into build/: workers-go's JavaScript, the Wasm, charter's glue (app/worker/*.mjs of the charter
// go.mod requires, so the JavaScript is the one written for the Go it runs), and intl's Worker modules.
func wasm(p *project.Project, limit int) error {
	if err := run.Run(p.Root, nil, "go", "run", "github.com/syumai/workers-go/cmd/workers-assets-gen", "-mode=go"); err != nil {
		return err
	}
	out := filepath.Join("build", "app.wasm")
	if err := run.Run(p.Root, map[string]string{"GOOS": "js", "GOARCH": "wasm"}, "go", "build", "-trimpath", "-ldflags=-s", "-o", out, "."); err != nil {
		return err
	}
	dir, err := run.Output(p.Root, nil, "go", "list", "-m", "-f", "{{.Dir}}", library)
	if err != nil {
		return fmt.Errorf("go.mod must require %s: %w", library, err)
	}
	glue, _ := filepath.Glob(filepath.Join(strings.TrimSpace(dir), "app", "worker", "*.mjs"))
	if len(glue) == 0 {
		return fmt.Errorf("no Worker glue (app/worker/*.mjs) in %s", dir)
	}
	for _, f := range glue {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(p.Root, "build", filepath.Base(f)), b, 0o644); err != nil {
			return err
		}
	}
	// The Worker's entry, in place of workers-go's (a Go runtime per request): charter's go.mjs keeps runtimes between
	// requests, and starts one while the module loads when charter.toml names a health path, which must touch no
	// binding. It exports Hub, the Durable Object class of charter's hub, for a Worker that binds one, and gives Go
	// the app's languages. An app with more of its own (another Durable Object, routes in JavaScript) has an entry of
	// its own that imports from ./build/ the same way.
	entry := "// Written by charter build: the Worker's entry (wrangler.jsonc's main).\nimport { goWorker } from \"./go.mjs\";\n"
	if hasIntl(p) {
		entry += "import { goIntl } from \"./intl-worker.mjs\";\n\n// The app's languages, for Go (charter's intl).\nglobalThis.goIntl = goIntl;\n"
	}
	entry += "\nexport { Hub } from \"./hub.mjs\";\n\nconst go = goWorker();\n"
	if p.Health != "" {
		entry += fmt.Sprintf("await go.warm({ paths: [%q] });\n", p.Health)
	}
	entry += "\n// Requests, Cron Triggers and queues, as workers-go's own entry has them, and email (charter/bindings).\nexport default { fetch: go.fetch, scheduled: go.scheduled, queue: go.queue, email: go.email };\n"
	if err := os.WriteFile(filepath.Join(p.Root, "build", "worker.mjs"), []byte(entry), 0o644); err != nil {
		return err
	}
	if hasIntl(p) {
		intl.Main([]string{"assets", "-worker", "build"})
	}
	b, err := os.ReadFile(filepath.Join(p.Root, out))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	z, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	z.Write(b)
	z.Close()
	say("✓ %s: %.2f MB, %.2f MB gzipped (Workers Free takes 3, Paid 10)", out, float64(len(b))/1e6, float64(buf.Len())/1e6)
	if limit > 0 && buf.Len() > limit {
		return fmt.Errorf("%s is %d bytes gzipped, over the limit of %d", out, buf.Len(), limit)
	}
	return nil
}

// pushSchema is what Void calls db push: the local database follows db/schema.sql as it is edited, and keeps its
// rows. The migration for what is not committed yet is written again from the schema, the queries' Go with it; then
// the local database is asked what it has, and gets exactly the difference to the schema, whatever migrations it had
// run before. Its record of migrations is set to the files there are, so that charter dev's next start has nothing to
// apply twice.
func pushSchema(p *project.Project) error {
	if len(p.Worker.D1) == 0 || p.DB == "" {
		return nil
	}
	// Locally a column may go with its data: charter check --fix asks before that is committed.
	if err := dbcode.Write(p.Root, p.DB, false, true); err != nil {
		return err
	}
	schema, err := os.ReadFile(filepath.Join(p.Root, "db", "schema.sql"))
	if err != nil {
		return nil
	}
	bin, err := npm(p, "wrangler")
	if err != nil {
		return err
	}
	binding := p.Worker.D1[0].Binding
	env := map[string]string{"CI": "1", "WRANGLER_SEND_METRICS": "false"}
	out, err := run.Output(p.Root, env, bin, "d1", "execute", binding, "--local", "--json", "--command",
		"SELECT sql FROM sqlite_master WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'd1_%' AND name NOT LIKE '_cf_%' ORDER BY CASE type WHEN 'table' THEN 0 WHEN 'index' THEN 1 ELSE 2 END")
	if err != nil {
		return err
	}
	var answer []struct {
		Results []struct {
			SQL string `json:"sql"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out[max(strings.Index(out, "["), 0):]), &answer); err != nil || len(answer) == 0 {
		return fmt.Errorf("the local database did not say what it has: %v", err)
	}
	var has strings.Builder
	for _, r := range answer[0].Results {
		has.WriteString(r.SQL + ";\n")
	}
	statements, _, notes, err := dbcode.Diff(has.String(), string(schema))
	if err != nil {
		return err
	}
	for _, n := range notes {
		say("· local database: %s", n)
	}
	// The migrations there are, as the ones that ran: wrangler's own record (d1_migrations).
	files, _ := filepath.Glob(filepath.Join(p.Root, p.Worker.D1[0].MigrationsDir(), "*.sql"))
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = "'" + filepath.Base(f) + "'"
	}
	statements = append(statements,
		"CREATE TABLE IF NOT EXISTS d1_migrations (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL);",
		"DELETE FROM d1_migrations WHERE name NOT IN ("+strings.Join(append(names, "''"), ", ")+");")
	for _, n := range names {
		statements = append(statements, "INSERT OR IGNORE INTO d1_migrations (name) VALUES ("+n+");")
	}
	file := filepath.Join(p.Root, ".charter", "push.sql")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(strings.Join(statements, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	_, err = run.Output(p.Root, env, bin, "d1", "execute", binding, "--local", "--file", file)
	return err
}
