// Package local writes what runs a Go Worker on plain workerd, no wrangler and no Node, from charter.toml: into
// .charter/ at the app's root, rebuilt by every command that needs it and never committed (.charter/.gitignore).
//
//	.charter/config.capnp   workerd's config: the app's modules and the build, its bindings, the port
//	.charter/local.mjs      the entry: the app's own entry, given what Cloudflare would give it and workerd can't
//	                        (a D1 database, rate limits, the visitor's time zone in request.cf)
//	.charter/d1.mjs         D1 on a Durable Object's SQLite, the migrations applied on first use
//	.charter/assets.mjs     Static Assets: a file from the assets folder when there is one, else the Worker
//
// workerd serve .charter/config.capnp runs it from the app's root (disk paths are relative to it).
package local

import (
	"fmt"
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

const header = "Written by `go tool charter local` from charter.toml: don't edit it, change charter.toml. Local only, never deployed."

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
	files := map[string]string{
		".gitignore":   "# " + header + "\n*\n",
		"config.capnp": config(p, build, migrations),
		"local.mjs":    entry(p),
		"d1.mjs":       d1(p, migrations),
		"assets.mjs":   assets,
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
		return nil, fmt.Errorf("no build/: build first (go tool charter build): %w", err)
	}
	var out []string
	for _, e := range entries {
		switch filepath.Ext(e.Name()) {
		case ".mjs", ".js", ".wasm":
			out = append(out, e.Name())
		}
	}
	if !slices.Contains(out, "worker.mjs") {
		return nil, fmt.Errorf("build/ has no worker.mjs: build first (go tool charter build)")
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

func config(p *project.Project, build []string, migrations []migration) string {
	var b strings.Builder
	q := strconv.Quote
	fmt.Fprintf(&b, "# %s\n#\n# workerd serve .charter/config.capnp, from the app's root. Embed paths are relative to this file, disk paths to\n# the working directory.\n\n", header)
	b.WriteString("using Workerd = import \"/workerd/workerd.capnp\";\n\nconst config :Workerd.Config = (\n  services = [\n")
	front := "app"
	if p.Assets != "" {
		front = "assets"
		fmt.Fprintf(&b, "    (name = \"assets\", worker = .assets),\n    (name = \"public\", disk = %s),\n", q(p.Assets))
	}
	b.WriteString("    (name = \"app\", worker = .app),\n")
	fmt.Fprintf(&b, "    (name = \"state\", disk = (path = %s, writable = true)),\n  ],\n", q(StateDir))
	fmt.Fprintf(&b, "  sockets = [ (name = \"http\", address = %s, http = (), service = %s) ],\n);\n\n", q("127.0.0.1:"+p.LocalPort()), q(front))
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
	for _, d := range p.DurableObjects {
		fmt.Fprintf(&b, "    (name = %s, durableObjectNamespace = %s),\n", q(d.Binding), q(d.Class))
	}
	if len(p.D1) > 0 {
		b.WriteString("    (name = \"CHARTER_D1\", durableObjectNamespace = \"CharterD1\"),\n")
	}
	b.WriteString("    # The developer's own zone, standing in for Cloudflare's request.cf.timezone: charter serve sets it, charter with\n" +
		"    # doesn't, so tests run in UTC on every machine.\n    (name = \"LOCAL_TIME_ZONE\", fromEnvironment = \"LOCAL_TIME_ZONE\"),\n")
	fmt.Fprintf(&b, "  ],\n  compatibilityDate = %s,\n);\n", q(p.CompatibilityDate))
	return b.String()
}

func entry(p *project.Project) string {
	var b strings.Builder
	q := strconv.Quote
	from := "../" + entryModule(p)
	fmt.Fprintf(&b, "// %s\n//\n// The app's entry (%s), given what Cloudflare gives a Worker and workerd alone doesn't.\n\n", header, entryModule(p))
	fmt.Fprintf(&b, "import entry from %s;\nexport * from %s;\n", q(from), q(from))
	if len(p.D1) > 0 {
		b.WriteString("import { database } from \"./d1.mjs\";\nexport { CharterD1 } from \"./d1.mjs\";\n")
	}
	b.WriteString(`
// A Workers Rate Limiting binding's fixed window, in memory: limit({ key }) → { success }.
function rateLimit(limit, period) {
  const windows = new Map();
  return {
    async limit({ key }) {
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
const rateLimits = {
`)
	for _, r := range p.RateLimits {
		fmt.Fprintf(&b, "  %s: rateLimit(%d, %d),\n", q(r.Binding), r.Limit, r.Period)
	}
	b.WriteString("};\n\nexport default {\n  ...entry,\n  fetch(request, env, ctx) {\n" +
		"    // Cloudflare gives the Worker each visitor's zone from their IP (request.cf.timezone); locally, the developer's.\n" +
		"    if (env.LOCAL_TIME_ZONE && !request.cf?.timezone) {\n" +
		"      request = new Request(request, { cf: { ...request.cf, timezone: env.LOCAL_TIME_ZONE } });\n    }\n" +
		"    const local = { ...rateLimits };\n")
	for _, d := range p.D1 {
		fmt.Fprintf(&b, "    local[%s] = database(env.CHARTER_D1, %s);\n", q(d.Binding), q(d.Binding))
	}
	b.WriteString("    return entry.fetch(request, { ...env, ...local }, ctx);\n  },\n};\n")
	return b.String()
}

func d1(p *project.Project, migrations []migration) string {
	var b strings.Builder
	q := strconv.Quote
	fmt.Fprintf(&b, "// %s\n//\n// D1 on a Durable Object's SQLite, one object per binding, its migrations applied on first use. The D1 API that Go's\n"+
		"// drivers call: prepare(sql).bind(...args) then run(), all(), first() or raw({ columnNames }).\n\n", header)
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
    sql.exec("CREATE TABLE IF NOT EXISTS _charter_migrations (name TEXT PRIMARY KEY)");
    for (const [name, body] of MIGRATIONS[binding] ?? []) {
      if (sql.exec("SELECT 1 FROM _charter_migrations WHERE name = ?", name).toArray().length) continue;
      sql.exec(body);
      sql.exec("INSERT INTO _charter_migrations (name) VALUES (?)", name);
    }
    this.migrated = true;
  }

  query(binding, query, params, mode) {
    this.migrate(binding);
    const sql = this.ctx.storage.sql;
    const cursor = sql.exec(query, ...params);
    if (mode === "raw") return [cursor.columnNames, ...cursor.raw().toArray()];
    const results = mode === "all" ? cursor.toArray() : [];
    const { changes, id } = sql.exec("SELECT changes() AS changes, last_insert_rowid() AS id").one();
    return { success: true, results, meta: { changes, last_row_id: id } };
  }
}

// database is binding's D1 database, on its own CharterD1 object.
export function database(namespace, binding) {
  const db = () => namespace.get(namespace.idFromName(binding));
  const statement = (query, params) => ({
    bind: (...args) => statement(query, args),
    run: () => db().query(binding, query, params, "run"),
    all: () => db().query(binding, query, params, "all"),
    raw: async (options) => {
      const [columns, ...rows] = await db().query(binding, query, params, "raw");
      return options?.columnNames ? [columns, ...rows] : rows;
    },
    first: async (column) => {
      const { results } = await db().query(binding, query, params, "all");
      const row = results[0] ?? null;
      return column === undefined || row === null ? row : row[column];
    },
  });
  return { prepare: (query) => statement(query, []) };
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
