// Package local writes what runs a Worker (Go or TypeScript) on plain workerd, no wrangler and no Node, from
// charter.toml: into .charter/ at the app's root, rebuilt by every command that needs it and never committed
// (.charter/.gitignore). What Cloudflare gives a Worker and workerd alone doesn't is given as Cloudflare gives it, as
// bindings, so the Worker sees it however it reads its env (the fetch's, or cloudflare:workers'):
//
//	.charter/config.capnp   workerd's config: the app's modules and the build, its bindings, the port
//	.charter/local.mjs      the entry: the app's own entry, given the visitor's time zone in request.cf
//	.charter/d1.mjs         D1 on a Durable Object's SQLite, the migrations applied on first use, answering what
//	                        workerd's own D1 client (cloudflare-internal:d1-api) asks, as a D1 binding's database
//	.charter/d1-entry.mjs   the service each D1 binding's client calls: its database's object
//	.charter/ratelimit.mjs  a Workers Rate Limiting binding, counting in memory
//	.charter/assets.mjs     Static Assets: a file from the assets folder when there is one, else the Worker
//
// workerd serve .charter/config.capnp runs it from the app's root (disk paths are relative to it).
package local

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/joeblew999/charter/project"
)

// Dir is where it writes, under the app's root.
const Dir = ".charter"

// StateDir is where local Durable Objects (D1's stand-in among them) keep their SQLite files, under the app's root.
const StateDir = Dir + "/state"

const header = "Written by `charter local` from charter.toml: don't edit it, change charter.toml. Local only, never deployed."

// Write writes .charter/ for p, from the build in build/.
func Write(p *project.Project) error {
	dir := filepath.Join(p.Root, Dir)
	if err := os.MkdirAll(filepath.Join(p.Root, StateDir), 0o755); err != nil {
		return err
	}
	build, err := buildModules(p.Root)
	if err != nil {
		return err
	}
	migrations, err := migrationModules(p)
	if err != nil {
		return err
	}
	limits, err := p.Limits()
	if err != nil {
		return err
	}
	files := map[string]string{
		".gitignore":    "# " + header + "\n*\n",
		"config.capnp":  config(p, build, migrations, limits),
		"local.mjs":     entry(p),
		"d1.mjs":        d1(p, migrations),
		"d1-entry.mjs":  d1Entry,
		"ratelimit.mjs": rateLimit,
		"assets.mjs":    assets,
	}
	for name, content := range files {
		if err := write(filepath.Join(dir, name), content); err != nil {
			return err
		}
	}
	return nil
}

// write writes content unless the file already holds it, so workerd's file watching sees no change.
func write(path, content string) error {
	if old, err := os.ReadFile(path); err == nil && string(old) == content {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// buildModules are the build's files workerd loads: its JavaScript and its Wasm.
func buildModules(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "build"))
	if err != nil {
		return nil, fmt.Errorf("no build/: build first (charter build): %w", err)
	}
	var out []string
	for _, e := range entries {
		switch filepath.Ext(e.Name()) {
		case ".mjs", ".js", ".wasm":
			out = append(out, e.Name())
		}
	}
	if !slices.Contains(out, "worker.mjs") {
		return nil, fmt.Errorf("build/ has no worker.mjs: build first (charter build)")
	}
	return out, nil
}

// migration is one D1 migration file: its module name and path, both relative to the app's root.
type migration struct{ binding, path string }

func migrationModules(p *project.Project) ([]migration, error) {
	var out []migration
	for _, d := range p.D1 {
		if d.Migrations == "" {
			continue
		}
		files, err := filepath.Glob(filepath.Join(p.Root, d.Migrations, "*.sql"))
		if err != nil {
			return nil, err
		}
		slices.Sort(files)
		for _, f := range files {
			rel, err := filepath.Rel(p.Root, f)
			if err != nil {
				return nil, err
			}
			out = append(out, migration{d.Binding, filepath.ToSlash(rel)})
		}
	}
	return out, nil
}

// entryModule is the app's entry module's name in workerd: what its own imports resolve against, as on Cloudflare
// (charter deploy names modules by file name, and the build's files build/<file>).
func entryModule(p *project.Project) string {
	if p.Main == "" {
		return "build/worker.mjs"
	}
	return filepath.Base(p.Main)
}

// d1Service is the name of the service binding's D1 client calls.
func d1Service(binding string) string { return "charter-d1-" + binding }

func config(p *project.Project, build []string, migrations []migration, limits []project.RateLimit) string {
	var b strings.Builder
	q := strconv.Quote
	fmt.Fprintf(&b, "# %s\n#\n# workerd serve .charter/config.capnp, from the app's root. Embed paths are relative to this file, disk paths to\n# the working directory.\n\n", header)
	b.WriteString("using Workerd = import \"/workerd/workerd.capnp\";\n\nconst config :Workerd.Config = (\n  services = [\n")
	// What the Worker's fetch reaches: workerd's own allows only the internet, where a local Worker calls what runs
	// beside it too (a webhook's receiver, a token issuer), as cf dev's allows.
	b.WriteString("    (name = \"internet\", network = (allow = [\"public\", \"private\", \"local\"], tlsOptions = (trustBrowserCas = true))),\n")
	front := "app"
	if p.Assets != "" {
		front = "assets"
		fmt.Fprintf(&b, "    (name = \"assets\", worker = .assets),\n    (name = \"public\", disk = %s),\n", q(p.Assets))
	}
	b.WriteString("    (name = \"app\", worker = .app),\n")
	for i, d := range p.D1 {
		fmt.Fprintf(&b, "    (name = %s, worker = .d1x%d),\n", q(d1Service(d.Binding)), i)
	}
	fmt.Fprintf(&b, "    (name = \"state\", disk = (path = %s, writable = true)),\n  ],\n", q(StateDir))
	fmt.Fprintf(&b, "  sockets = [ (name = \"http\", address = %s, http = (), service = %s) ],\n", q("127.0.0.1:"+p.LocalPort()), q(front))
	if len(limits) > 0 {
		// The module behind each rate limit binding: one of workerd's wrapped bindings, made from an internal module.
		b.WriteString("  extensions = [ (modules = [ (name = \"charter-internal:ratelimit\", esModule = embed \"ratelimit.mjs\", internal = true) ]) ],\n")
	}
	b.WriteString(");\n\n")
	for i, d := range p.D1 {
		fmt.Fprintf(&b, "const d1x%d :Workerd.Worker = (\n  modules = [ (name = \"d1-entry.mjs\", esModule = embed \"d1-entry.mjs\") ],\n"+
			"  bindings = [ (name = \"D1\", durableObjectNamespace = (className = \"CharterD1\", serviceName = \"app\")), (name = \"BINDING\", text = %s) ],\n"+
			"  compatibilityDate = %s,\n);\n\n", i, q(d.Binding), q(p.CompatibilityDate))
	}
	if p.Assets != "" {
		fmt.Fprintf(&b, "const assets :Workerd.Worker = (\n  modules = [ (name = \"assets.mjs\", esModule = embed \"assets.mjs\") ],\n"+
			"  bindings = [ (name = \"ASSETS\", service = \"public\"), (name = \"APP\", service = \"app\") ],\n  compatibilityDate = %s,\n);\n\n", q(p.CompatibilityDate))
	}
	b.WriteString("const app :Workerd.Worker = (\n  modules = [\n")
	b.WriteString("    (name = \"charter/local.mjs\", esModule = embed \"local.mjs\"),\n")
	if len(p.D1) > 0 {
		b.WriteString("    (name = \"charter/d1.mjs\", esModule = embed \"d1.mjs\"),\n")
		seen := map[string]bool{}
		for _, m := range migrations {
			if !seen[m.path] {
				seen[m.path] = true
				fmt.Fprintf(&b, "    (name = %s, text = embed %s),\n", q(m.path), q("../"+m.path))
			}
		}
	}
	if p.Main != "" {
		fmt.Fprintf(&b, "    (name = %s, esModule = embed %s),\n", q(entryModule(p)), q("../"+filepath.ToSlash(p.Main)))
		for _, m := range p.Modules {
			fmt.Fprintf(&b, "    (name = %s, esModule = embed %s),\n", q(filepath.Base(m)), q("../"+filepath.ToSlash(m)))
		}
	}
	for _, f := range build {
		kind := "esModule"
		if filepath.Ext(f) == ".wasm" {
			kind = "wasm"
		}
		fmt.Fprintf(&b, "    (name = %s, %s = embed %s),\n", q("build/"+f), kind, q("../build/"+f))
	}
	b.WriteString("  ],\n  durableObjectNamespaces = [\n")
	for _, d := range p.DurableObjects {
		fmt.Fprintf(&b, "    (className = %s, uniqueKey = %s, enableSql = true),\n", q(d.Class), q(p.Name+"-"+d.Class))
	}
	if len(p.D1) > 0 {
		fmt.Fprintf(&b, "    (className = \"CharterD1\", uniqueKey = %s, enableSql = true),\n", q(p.Name+"-charter-d1"))
	}
	b.WriteString("  ],\n  durableObjectStorage = (localDisk = \"state\"),\n  bindings = [\n")
	for _, k := range sortedKeys(p.Local.Vars) {
		fmt.Fprintf(&b, "    (name = %s, text = %s),\n", q(k), q(p.Local.Vars[k]))
	}
	// A secret without a local value in [local] vars, and what [test] env gives the Worker under test: from workerd's
	// environment (charter check sets them for its run). Only one the environment has is bound: workerd binds an unset
	// one as null, where Cloudflare (and cf dev) leave an unset secret out, and the Worker must see the same.
	var fromEnv []string
	for _, k := range append(append(slices.Clone(p.Secrets.Required), p.Secrets.Optional...), slices.Sorted(maps.Keys(p.Test.Env))...) {
		if _, ok := p.Local.Vars[k]; !ok && os.Getenv(k) != "" && !slices.Contains(fromEnv, k) {
			fromEnv = append(fromEnv, k)
		}
	}
	for _, k := range fromEnv {
		fmt.Fprintf(&b, "    (name = %s, fromEnvironment = %s),\n", q(k), q(k))
	}
	for _, d := range p.DurableObjects {
		fmt.Fprintf(&b, "    (name = %s, durableObjectNamespace = %s),\n", q(d.Binding), q(d.Class))
	}
	// Each D1 database is workerd's own D1 client, calling its service (d1-entry.mjs, d1.mjs).
	for _, d := range p.D1 {
		fmt.Fprintf(&b, "    (name = %s, wrapped = (moduleName = \"cloudflare-internal:d1-api\", innerBindings = [ (name = \"fetcher\", service = %s) ])),\n",
			q(d.Binding), q(d1Service(d.Binding)))
	}
	for _, r := range limits {
		fmt.Fprintf(&b, "    (name = %s, wrapped = (moduleName = \"charter-internal:ratelimit\", innerBindings = [ (name = \"limit\", json = %s), (name = \"period\", json = %s) ])),\n",
			q(r.Binding), q(strconv.Itoa(r.Limit)), q(strconv.Itoa(r.Period)))
	}
	b.WriteString("    # The developer's own zone, standing in for Cloudflare's request.cf.timezone: charter serve sets it, charter with\n" +
		"    # doesn't, so tests run in UTC on every machine.\n    (name = \"LOCAL_TIME_ZONE\", fromEnvironment = \"LOCAL_TIME_ZONE\"),\n")
	fmt.Fprintf(&b, "  ],\n  compatibilityDate = %s,\n", q(p.CompatibilityDate))
	if len(p.CompatibilityFlags) > 0 {
		var flags []string
		for _, f := range p.CompatibilityFlags {
			flags = append(flags, q(f))
		}
		fmt.Fprintf(&b, "  compatibilityFlags = [%s],\n", strings.Join(flags, ", "))
	}
	b.WriteString(");\n")
	return b.String()
}

// entry is local.mjs: the app's own entry, given the visitor's time zone in request.cf, and charter's D1 class.
func entry(p *project.Project) string {
	var b strings.Builder
	q := strconv.Quote
	from := "../" + entryModule(p)
	fmt.Fprintf(&b, "// %s\n//\n// The app's entry (%s), given what Cloudflare gives a Worker's request and workerd alone doesn't.\n\n", header, entryModule(p))
	fmt.Fprintf(&b, "import entry from %s;\nexport * from %s;\n", q(from), q(from))
	if len(p.D1) > 0 {
		b.WriteString("export { CharterD1 } from \"./d1.mjs\";\n")
	}
	b.WriteString("\nexport default {\n  ...entry,\n  fetch(request, env, ctx) {\n" +
		"    // Cloudflare gives the Worker each visitor's zone from their IP (request.cf.timezone); locally, the developer's.\n" +
		"    if (env.LOCAL_TIME_ZONE && !request.cf?.timezone) {\n" +
		"      request = new Request(request, { cf: { ...request.cf, timezone: env.LOCAL_TIME_ZONE } });\n    }\n" +
		"    return entry.fetch(request, env, ctx);\n  },\n};\n")
	return b.String()
}

// rateLimit is ratelimit.mjs: a Workers Rate Limiting binding (one of workerd's wrapped bindings, made from its limit
// and period), a fixed window per key, in memory, as Cloudflare's counts: limit({ key }) → { success }.
const rateLimit = "// " + header + `
//
// A Workers Rate Limiting binding, in memory: a fixed window of limit calls per period seconds, per key.

export default function ({ limit, period }) {
  const windows = new Map();
  return {
    async limit({ key = "" } = {}) {
      const now = Date.now();
      const w = windows.get(key);
      if (!w || now - w.start >= period * 1000) {
        windows.set(key, { start: now, count: 1 });
        return { success: true };
      }
      w.count++;
      return { success: w.count <= limit };
    },
  };
}
`

// d1Entry is d1-entry.mjs: the service a D1 binding's client (workerd's own) calls, which hands each request to the
// binding's database, its own CharterD1 object.
const d1Entry = "// " + header + `
//
// The service a D1 binding's client calls: every request to its database, the binding's own CharterD1 object.

export default {
  fetch(request, env) {
    const headers = new Headers(request.headers);
    headers.set("x-charter-d1-binding", env.BINDING);
    return env.D1.get(env.D1.idFromName(env.BINDING)).fetch(new Request(request, { headers }));
  },
};
`

func d1(p *project.Project, migrations []migration) string {
	var b strings.Builder
	q := strconv.Quote
	fmt.Fprintf(&b, "// %s\n//\n// D1 on a Durable Object's SQLite, one object per binding, its migrations applied on first use. It answers what\n"+
		"// workerd's own D1 client (cloudflare-internal:d1-api, the binding) asks of a database, as Cloudflare's does: POST\n"+
		"// /query or /execute, a statement or a batch ({ sql, params }), in one transaction, each result with its meta.\n\n", header)
	b.WriteString("import { DurableObject } from \"cloudflare:workers\";\n")
	for i, m := range migrations {
		fmt.Fprintf(&b, "import m%d from %s;\n", i, q("../"+m.path))
	}
	b.WriteString("\nconst MIGRATIONS = {\n")
	for _, d := range p.D1 {
		var list []string
		for i, m := range migrations {
			if m.binding == d.Binding {
				list = append(list, fmt.Sprintf("[%s, m%d]", q(m.path), i))
			}
		}
		fmt.Fprintf(&b, "  %s: [%s],\n", q(d.Binding), strings.Join(list, ", "))
	}
	b.WriteString(`};

export class CharterD1 extends DurableObject {
  migrate(binding) {
    if (this.migrated) return;
    const sql = this.ctx.storage.sql;
    // Each migration once, by its file name in d1_migrations, as on Cloudflare (charter deploy, cf d1 migrations).
    sql.exec("CREATE TABLE IF NOT EXISTS d1_migrations(id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL)");
    // Local state charter migrated before, into _charter_migrations by path, keeps what it has.
    if (sql.exec("SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = '_charter_migrations'").toArray().length) {
      for (const { name } of sql.exec("SELECT name FROM _charter_migrations").toArray()) {
        sql.exec("INSERT OR IGNORE INTO d1_migrations (name) VALUES (?)", name.split("/").pop());
      }
    }
    for (const [path, body] of MIGRATIONS[binding] ?? []) {
      const name = path.split("/").pop();
      if (sql.exec("SELECT 1 FROM d1_migrations WHERE name = ?", name).toArray().length) continue;
      sql.exec(body);
      sql.exec("INSERT INTO d1_migrations (name) VALUES (?)", name);
    }
    this.migrated = true;
  }

  // What D1's client sends: its statements, run in one transaction; their results, or { success: false, error }.
  async fetch(request) {
    const url = new URL(request.url);
    if (request.method !== "POST" || (url.pathname !== "/query" && url.pathname !== "/execute")) {
      return new Response("not a D1 request", { status: 404 });
    }
    try {
      this.migrate(request.headers.get("x-charter-d1-binding"));
      let queries = await request.json();
      if (!Array.isArray(queries)) queries = [queries];
      queries = queries.filter((query) => query.sql.replace(/^\s+--.*/gm, "").trim().length > 0);
      if (queries.length === 0) throw new Error("No SQL statements detected.");
      const format = url.searchParams.get("resultsFormat") === "ROWS_AND_COLUMNS" ? "ROWS_AND_COLUMNS" : "ARRAY_OF_OBJECTS";
      const results = this.ctx.storage.transactionSync(() => queries.map((query) => this.run(query, format)));
      const headers = {};
      try {
        headers["x-cf-d1-session-commit-token"] = await this.ctx.storage.getCurrentBookmark();
      } catch {}
      return Response.json(results, { headers });
    } catch (e) {
      return Response.json({ success: false, error: e instanceof Error ? e.message : String(e) });
    }
  }

  // run runs one statement: its rows (as objects, or columns and rows), and the meta D1 gives with them.
  run(query, format) {
    const sql = this.ctx.storage.sql;
    const counts = () => sql.exec("SELECT total_changes() AS total, last_insert_rowid() AS id").one();
    const before = counts();
    const sizeBefore = sql.databaseSize;
    // A blob comes as an array of bytes.
    const params = (query.params ?? []).map((p) => (Array.isArray(p) ? new Uint8Array(p).buffer : p));
    const start = performance.now();
    const cursor = sql.exec(query.sql, ...params);
    const columns = cursor.columnNames;
    const rows = [...cursor.raw()].map((row) => row.map((v) => (v instanceof ArrayBuffer ? [...new Uint8Array(v)] : v)));
    const duration = performance.now() - start;
    const after = counts();
    const size = sql.databaseSize;
    const results = format === "ROWS_AND_COLUMNS" ? { columns, rows } : rows.map((row) => Object.fromEntries(columns.map((name, i) => [name, row[i]])));
    return {
      success: true,
      results,
      meta: {
        served_by: "charter.local",
        served_by_primary: true,
        total_attempts: 1,
        timings: { sql_duration_ms: duration },
        duration,
        changes: after.total - before.total,
        last_row_id: after.id,
        changed_db: after.total !== before.total || after.id !== before.id || size !== sizeBefore,
        size_after: size,
        rows_read: cursor.rowsRead,
        rows_written: cursor.rowsWritten,
      },
    };
  }
}
`)
	return b.String()
}

// assets stands in for Static Assets: a file from the assets folder when one is there, else the Worker, as Cloudflare
// does by default (run_worker_first = false). Only paths that look like files ("/app.css"); "/" goes to the Worker.
const assets = "// " + header + `
//
// Static Assets: a file from the assets folder when one is there, else the Worker (Cloudflare's default,
// run_worker_first = false). Only paths that look like files ("/app.css"); "/" and routes go to the Worker.

const TYPES = {
  css: "text/css; charset=utf-8",
  html: "text/html; charset=utf-8",
  ico: "image/x-icon",
  js: "text/javascript; charset=utf-8",
  json: "application/json",
  mjs: "text/javascript; charset=utf-8",
  png: "image/png",
  svg: "image/svg+xml",
  txt: "text/plain; charset=utf-8",
  webp: "image/webp",
  woff2: "font/woff2",
};

export default {
  async fetch(request, env) {
    const { pathname } = new URL(request.url);
    const name = pathname.split("/").pop();
    if ((request.method === "GET" || request.method === "HEAD") && name.includes(".")) {
      const file = await env.ASSETS.fetch("http://assets" + pathname, { method: request.method });
      if (file.ok) {
        const type = TYPES[name.split(".").pop().toLowerCase()] ?? "application/octet-stream";
        return new Response(file.body, { headers: { "Content-Type": type } });
      }
    }
    return env.APP.fetch(request);
  },
};
`

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
