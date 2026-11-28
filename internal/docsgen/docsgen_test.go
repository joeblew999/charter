package docsgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "docs", "index.md"), []byte("# Start\n\nWhat `it` is. See [the pages](02-pages.md#routes).\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "docs", "02-pages.md"), []byte("# Pages\n\nHow pages work.\n\n## Routes\n\n```go\nx := 1\n```\n\nBack to [the start](index.md).\n"), 0o644)
	n, err := Write(dir)
	if err != nil || n != 2 {
		t.Fatal(n, err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "docs", "docs.built.json"))
	code := string(b)
	for _, want := range []string{`"Title": "Start"`, `"Slug": "pages"`, `"Description": "How pages work."`, `"Description": "What it is. See the pages."`, `"ID": "routes"`,
		`href=\"pages#routes\"`, `href=\"./\"`, `routes\"\u003eRoutes\u003c/h2`, `pre class=\"chroma\"`, `span class=\"mi\"\u003e1\u003c/span`} {
		if !strings.Contains(code, want) {
			t.Errorf("no %s in:\n%s", want, code)
		}
	}
	if strings.Index(code, `"Title": "Start"`) > strings.Index(code, `"Title": "Pages"`) {
		t.Error("index.md is not first")
	}
}

// A sample that names a file is held to it: Samples says when the Markdown's copy differs, writes it again with fix,
// takes one declaration with its comment when one is named, and the page says which file the code is from.
func TestSamples(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.MkdirAll(filepath.Join(dir, "pages"), 0o755)
	os.WriteFile(filepath.Join(dir, "pages", "pages.go"), []byte("package pages\n\n// Home is the front page.\ntype Home struct{}\n\n// Props loads it.\nfunc (Home) Props() (int, error) { return 1, nil }\n\nfunc other() {}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "pages", "home.gsx"), []byte("package pages\n\n// Page is the page.\ncomponent (p Home) Page(n int) {\n\t<p>{ n }</p>\n}\n\ncomponent Other() {\n\t<p/>\n}\n"), 0o644)
	page := filepath.Join(dir, "docs", "index.md")
	os.WriteFile(page, []byte("# Start\n\n```go file=pages/pages.go#Home.Props\nold\n```\n\n```gsx file=pages/home.gsx#Home.Page\nold\n```\n\n```go\nuntouched := 1\n```\n"), 0o644)
	differ, err := Samples(dir, false)
	if err != nil || len(differ) != 2 {
		t.Fatalf("%v %v", differ, err)
	}
	if b, _ := os.ReadFile(page); !strings.Contains(string(b), "old") {
		t.Error("without fix the page was written")
	}
	if _, err := Samples(dir, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(page)
	for _, want := range []string{"```go file=pages/pages.go#Home.Props\n// Props loads it.\nfunc (Home) Props() (int, error) { return 1, nil }\n```", "```gsx file=pages/home.gsx#Home.Page\n// Page is the page.\ncomponent (p Home) Page(n int) {\n\t<p>{ n }</p>\n}\n```", "untouched := 1"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("no %q in:\n%s", want, b)
		}
	}
	if strings.Contains(string(b), "other") || strings.Contains(string(b), "Other") {
		t.Errorf("more than the declaration named:\n%s", b)
	}
	if differ, err := Samples(dir, false); err != nil || len(differ) != 0 {
		t.Errorf("after fix: %v %v", differ, err)
	}
	if _, err := Write(dir); err != nil {
		t.Fatal(err)
	}
	if built, _ := os.ReadFile(filepath.Join(dir, "docs", "docs.built.json")); !strings.Contains(string(built), "figcaption\\u003epages/pages.go") {
		t.Errorf("the page does not say which file the code is from:\n%s", built)
	}
	os.WriteFile(page, []byte("# Start\n\n```go file=pages/pages.go#Gone\n```\n"), 0o644)
	if _, err := Samples(dir, false); err == nil {
		t.Error("a sample of a declaration that is not there passed")
	}
}

// An app's docs have the pages charter writes of the app itself after the ones a person wrote: the API from its
// contract, the database from its schema and queries, what the Worker is bound to from wrangler.jsonc. Each is
// there when its source is, says what it was written from, and links only to parts of itself that are there.
func TestReference(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/index.md", "# An app\n\nWhat it is for.\n")
	if n, err := Write(dir); err != nil || n != 1 {
		t.Fatalf("with nothing to write of: %d pages, %v", n, err)
	}
	write("wrangler.jsonc", `{"name": "x", "vars": {"APP_NAME": "x"}, "d1_databases": [{"binding": "DB"}]}`)
	write("db/schema.sql", "CREATE TABLE notes (\n  id INTEGER PRIMARY KEY AUTOINCREMENT,\n  body TEXT NOT NULL, -- doc: What it says\n  seen_at TEXT\n);\n")
	write("db/queries.sql", "-- The notes' queries.\n\n-- name: Note :one\n-- Note is one note, by its id.\nSELECT * FROM notes WHERE id = ?;\n\n-- name: Forget :exec\nDELETE FROM notes WHERE id = ?;\n")
	write("fern/openapi.json", `{"info": {"title": "x"}, "servers": [{"url": "https://x.example"}],
		"components": {"schemas": {"Note": {"type": "object", "required": ["id"], "properties": {"id": {"type": "integer", "format": "int64"}, "body": {"type": "string", "description": "What it says"}}}},
			"securitySchemes": {"bearer": {"type": "http", "scheme": "bearer"}}},
		"paths": {"/api/notes/{id}": {
			"get": {"operationId": "getNote", "summary": "One note", "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
				"responses": {"200": {"description": "OK", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Note"}}}}}},
			"delete": {"operationId": "removeNote", "summary": "Remove a note", "security": [{"bearer": ["write"]}], "x-rate-limit": {"binding": "WRITES", "limit": 10, "period": 60},
				"requestBody": {"content": {"application/json": {"schema": {"type": "object", "required": ["why"], "properties": {"why": {"type": "string"}}}}}},
				"responses": {"204": {"description": "No Content"}}}}}}`)
	if n, err := Write(dir); err != nil || n != 4 {
		t.Fatalf("with an API, a database and bindings: %d pages, %v", n, err)
	}
	built, _ := os.ReadFile(filepath.Join(dir, "docs", "docs.built.json"))
	var pages []struct{ Slug, Title, Markdown, HTML, Source string }
	if err := json.Unmarshal(built, &pages); err != nil {
		t.Fatal(err)
	}
	by := map[string]int{}
	for i, p := range pages {
		by[p.Slug] = i
	}
	for slug, want := range map[string][]string{
		"reference-api": {
			"# The API of x", "fern/openapi.json", "](/api/openapi.json)", "https://x.example",
			"| [`GET /api/notes/{id}`](#get-apinotesid) | One note | anyone | no limit of its own |",
			"| [`DELETE /api/notes/{id}`](#delete-apinotesid) | Remove a note | `write` | 10 calls in 60 s, for each caller |",
			"| `id` | path | integer | Required. |", "| 200 OK | [Note](#note) (`application/json`) |",
			"Its body is an object (`application/json`):", "| `why` | string |  |",
			"### Note", "| `id` | integer (int64) |  |", "| `body` | string | Optional. What it says |",
		},
		"reference-database": {
			"### notes", "| `id` | INTEGER | the key |  |", "| `body` | TEXT |  | What it says |", "| `seen_at` | TEXT | may be empty (NULL) |  |",
			"| `db.Note` | one row, or no such row | Note is one note, by its id. |", "| `db.Forget` | nothing | `DELETE FROM notes WHERE id = ?` |",
		},
		"reference-bindings": {"`env.DB`", "`env.APP_NAME`", "wrangler.jsonc"},
	} {
		i, ok := by[slug]
		if !ok {
			t.Errorf("no page at %s", slug)
			continue
		}
		for _, w := range want {
			if !strings.Contains(pages[i].Markdown, w) {
				t.Errorf("%s lacks %q:\n%s", slug, w, pages[i].Markdown)
			}
		}
	}
	// A link to a part of the page names an id the page has.
	api := pages[by["reference-api"]]
	for _, id := range []string{"get-apinotesid", "delete-apinotesid", "note"} {
		if !strings.Contains(api.HTML, `id="`+id+`"`) {
			t.Errorf("the API's page links to #%s and has no such part", id)
		}
	}
	if api.Source != "fern/openapi.json" || pages[by["reference-database"]].Source != "db/schema.sql" {
		t.Errorf("where they are written from: %q, %q", api.Source, pages[by["reference-database"]].Source)
	}
	// A page a person wrote where charter writes one is said, and nothing is written over.
	write("docs/05-reference-api.md", "# My own\n")
	if _, err := Write(dir); err == nil || !strings.Contains(err.Error(), "reference-api") {
		t.Errorf("a file at the same place: %v", err)
	}
}
