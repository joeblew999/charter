package envcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func app(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// What a Worker declares is written as a value of its kind, under its own name: the bindings and variables of
// wrangler.jsonc (comments and all), and the secrets .dev.vars.example and fnox.toml name. A rate limit carries
// the calls and the seconds wrangler.jsonc says, so they are said once.
func TestGo(t *testing.T) {
	dir := app(t, map[string]string{
		"wrangler.jsonc": `{
			// the Worker
			"name": "x",
			"vars": { "APP_NAME": "x", "MAIL_FROM": "a@b.c", },
			"d1_databases": [{ "binding": "DB" }],
			"kv_namespaces": [{ "binding": "KV" }],
			"r2_buckets": [{ "binding": "BUCKET" }],
			"queues": { "producers": [{ "binding": "QUEUE" }], "consumers": [{ "queue": "x" }] },
			"durable_objects": { "bindings": [{ "name": "HUB", "class_name": "Hub" }] },
			"ratelimits": [{ "name": "WRITE_LIMIT", "namespace_id": "1", "simple": { "limit": 100, "period": 60 } }],
			"send_email": [{ "name": "MAIL" }]
		}`,
		".dev.vars.example": "# local values\nWRITE_TOKEN=local\nMAIL_FROM=local@b.c\n",
		"fnox.toml":         "[secrets]\nWRITE_TOKEN = { provider = \"keychain\" }\nADMIN_TOKEN = { provider = \"keychain\" }\n\n[profiles.staging.secrets]\nSTAGING_TOKEN = { provider = \"keychain\" }\n",
	})
	got, err := Go(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"package env\n",
		`DB = cloud.BoundDatabase("DB")`,
		`KV = cloud.BoundKV("KV")`,
		`BUCKET = cloud.BoundBucket("BUCKET")`,
		`QUEUE = cloud.BoundQueue("QUEUE")`,
		`HUB = cloud.BoundObject("HUB")`,
		`WRITE_LIMIT = cloud.BoundRateLimit("WRITE_LIMIT", 100, 60)`,
		`MAIL = cloud.BoundMailer("MAIL")`,
		`APP_NAME  = cloud.BoundVariable("APP_NAME")`,
		`ADMIN_TOKEN   = cloud.BoundVariable("ADMIN_TOKEN")`,
		`STAGING_TOKEN = cloud.BoundVariable("STAGING_TOKEN")`,
		`WRITE_TOKEN   = cloud.BoundVariable("WRITE_TOKEN")`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the Go lacks %q:\n%s", want, got)
		}
	}
	// A variable with a local value among the secrets is declared once, as the variable.
	if strings.Count(got, `BoundVariable("MAIL_FROM")`) != 1 {
		t.Errorf("MAIL_FROM is not there once:\n%s", got)
	}
}

// A name that is no name in Go, and one declared as two kinds of thing, are said, with the name.
func TestNamesThatCannotBe(t *testing.T) {
	for name, wrangler := range map[string]string{
		"my-db": `{"name": "x", "d1_databases": [{ "binding": "my-db" }]}`,
		"db":    `{"name": "x", "d1_databases": [{ "binding": "db" }]}`,
		"DATA":  `{"name": "x", "d1_databases": [{ "binding": "DATA" }], "kv_namespaces": [{ "binding": "DATA" }]}`,
	} {
		if _, err := Go(app(t, map[string]string{"wrangler.jsonc": wrangler})); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A Worker that declares nothing has no file, and one that was there is taken away; a file that would not change is
// not written again, since a file that changes is a build.
func TestWrite(t *testing.T) {
	dir := app(t, map[string]string{"wrangler.jsonc": `{"name": "x", "vars": {"APP_NAME": "x"}}`})
	if err := Write(dir); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, File)
	first, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(dir); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.Stat(file); !again.ModTime().Equal(first.ModTime()) {
		t.Error("a file that had not changed was written again")
	}
	os.WriteFile(filepath.Join(dir, "wrangler.jsonc"), []byte(`{"name": "x"}`), 0o644)
	if err := Write(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("the file of a Worker that declares nothing is still there: %v", err)
	}
}

// A name written as text is found in each of the forms it has: workers-go's own functions under any import name, a
// call of cloud.Var, and the functions that make a binding's value. cloud.Var handed on as a function is not one,
// and generated files and another module's are not looked at.
func TestLint(t *testing.T) {
	dir := app(t, map[string]string{
		"built/mail.go":    "package built\n\nimport (\n\tcf \"github.com/syumai/workers-go/cloudflare\"\n\t\"github.com/syumai/workers-go/cloudflare/kv\"\n)\n\nvar from = cf.Getenv(\"MAIL_FROM\")\n\nfunc store() (*kv.Namespace, error) { return kv.NewNamespace(\"KV\") }\n",
		"api/auth.go":      "package api\n\nimport \"github.com/joeblew999/charter/cloud\"\n\nvar setting = cloud.Var\n\nvar name = cloud.Var(\"APP_NAME\")\n\nvar db = cloud.BoundDatabase(\"DB\")\n",
		"api/fine.go":      "package api\n\nimport \"github.com/joeblew999/charter/cloud\"\n\nfunc open(db cloud.Database) (cloud.DB, error) { return db.Open() }\n",
		"env/env.built.go": "package env\n\nimport \"github.com/joeblew999/charter/cloud\"\n\nvar DB = cloud.BoundDatabase(\"DB\")\n",
		"other/go.mod":     "module other\n",
		"other/x.go":       "package other\n\nimport \"github.com/joeblew999/charter/cloud\"\n\nvar db = cloud.BoundDatabase(\"DB\")\n",
	})
	got, err := Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"api/auth.go:7: cloud.Var(…) (env.NAME.Value())",
		"api/auth.go:9: cloud.BoundDatabase (the binding's value in env)",
		"built/mail.go:10: kv.NewNamespace (env.NAME.Open())",
		"built/mail.go:8: cloudflare.Getenv (env.NAME.Value())",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("found:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
