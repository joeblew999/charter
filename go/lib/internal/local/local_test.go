package local_test

import (
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/joeblew999/charter/go/lib/internal/local"
	"github.com/joeblew999/charter/go/lib/project"
)

// app is a Worker written in JavaScript standing in for a Go build: build/worker.mjs is what the Go build writes,
// and it answers from the bindings charter's local setup gives it.
var app = map[string]string{
	"charter.toml": `
name = "probe"
compatibility_date = "2026-09-11"
main = "worker/index.mjs"
modules = ["worker/counter.mjs"]
assets = "public"
health = "/healthz"

[[d1]]
binding = "DB"
migrations = "migrations"

[[durable_objects]]
binding = "COUNTER"
class = "Counter"
migration = "v1"

[[rate_limits]]
binding = "WRITES"
namespace_id = 1001
limit = 2
period = 60

[local]
vars = { APP_ENV = "local" }
`,
	"migrations/0001_notes.sql": "CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT NOT NULL);\n",
	"migrations/0002_seed.sql":  "INSERT INTO notes (body) VALUES ('first');\n",
	"public/hello.txt":          "from the assets\n",
	"worker/counter.mjs": `import { DurableObject } from "cloudflare:workers";
export class Counter extends DurableObject {
  async add() { const n = ((await this.ctx.storage.get("n")) ?? 0) + 1; await this.ctx.storage.put("n", n); return n; }
}
`,
	"worker/index.mjs": `import { handle } from "./build/worker.mjs";
export { Counter } from "./counter.mjs";
export default { fetch: handle };
`,
	"build/worker.mjs": `export async function handle(request, env) {
  const { pathname } = new URL(request.url);
  switch (pathname) {
    case "/healthz": return new Response("ok");
    case "/env": return new Response(env.APP_ENV);
    case "/zone": return new Response(request.cf?.timezone ?? "none");
    case "/notes": {
      await env.DB.prepare("INSERT INTO notes (body) VALUES (?)").bind("second").run();
      const { results } = await env.DB.prepare("SELECT body FROM notes ORDER BY id").bind().all();
      const raw = await env.DB.prepare("SELECT count(*) AS n FROM notes").bind().raw({ columnNames: true });
      return Response.json({ bodies: results.map((r) => r.body), raw });
    }
    case "/write": {
      const { success } = await env.WRITES.limit({ key: "me" });
      return new Response(success ? "ok" : "limited", { status: success ? 200 : 429 });
    }
    case "/count": return new Response(String(await env.COUNTER.get(env.COUNTER.idFromName("x")).add()));
  }
  return new Response("not found", { status: 404 });
}
`,
	"build/app.wasm": "\x00asm\x01\x00\x00\x00",
}

func writeApp(t *testing.T) *project.Project {
	t.Helper()
	root := t.TempDir()
	for name, content := range app {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := local.Write(p); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestWrite: the config names every module the app and the build have, each binding, and the port.
func TestWrite(t *testing.T) {
	t.Setenv("PORT", "")
	p := writeApp(t)
	b, err := os.ReadFile(filepath.Join(p.Root, local.Dir, "config.capnp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`(name = "index.mjs", esModule = embed "../worker/index.mjs")`, `(name = "counter.mjs", esModule = embed "../worker/counter.mjs")`,
		`(name = "build/worker.mjs", esModule = embed "../build/worker.mjs")`, `(name = "build/app.wasm", wasm = embed "../build/app.wasm")`,
		`(name = "migrations/0001_notes.sql", text = embed "../migrations/0001_notes.sql")`, `(name = "public", disk = "public")`,
		`(className = "Counter", uniqueKey = "probe-Counter", enableSql = true)`, `(name = "APP_ENV", text = "local")`,
		`address = "127.0.0.1:8787"`, `compatibilityDate = "2026-09-11"`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("config.capnp has no %s:\n%s", want, b)
		}
	}
	if g, _ := os.ReadFile(filepath.Join(p.Root, local.Dir, ".gitignore")); !strings.HasSuffix(string(g), "\n*\n") {
		t.Errorf(".gitignore: %q", g)
	}
}

// TestWorkerd: on workerd itself, the app gets its D1 database with the migrations applied, its rate limit, its
// Durable Object, its variables, the time zone, and the assets in front of it.
func TestWorkerd(t *testing.T) {
	bin, err := exec.LookPath("workerd")
	if err != nil || exec.Command(bin, "--version").Run() != nil {
		t.Skip("no workerd that runs on PATH")
	}
	p := writeApp(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	cmd := exec.Command(bin, "serve", local.Dir+"/config.capnp", "--socket-addr", "http=127.0.0.1:"+strconv.Itoa(port))
	cmd.Dir = p.Root
	cmd.Env = append(os.Environ(), "LOCAL_TIME_ZONE=Asia/Bangkok")
	var log strings.Builder
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Signal(syscall.SIGTERM); cmd.Wait() }()
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	get := func(path string) (int, string) {
		t.Helper()
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v\n%s", path, err, log.String())
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		if resp, err := http.Get(base + "/healthz"); err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("workerd didn't start:\n%s", log.String())
		}
	}
	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{"/env", 200, "local"},
		{"/zone", 200, "Asia/Bangkok"},
		{"/hello.txt", 200, "from the assets\n"},
		{"/notes", 200, `{"bodies":["first","second"],"raw":[["n"],[2]]}`},
		{"/count", 200, "1"},
		{"/count", 200, "2"},
		{"/write", 200, "ok"},
		{"/write", 200, "ok"},
		{"/write", 429, "limited"},
	} {
		if status, body := get(tc.path); status != tc.status || body != tc.body {
			t.Errorf("GET %s: %d %q, want %d %q\n%s", tc.path, status, body, tc.status, tc.body, log.String())
		}
	}
}
