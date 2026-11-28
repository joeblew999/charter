package docsgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
		`href=\"pages#routes\"`, `href=\"./\"`, `id=\"routes\" class=\"scroll-mt-20\"\u003eRoutes\u003c/h2`, `\u003cpre\u003e\u003ccode\u003e`, `span class=\"ts-number\"\u003e1\u003c/span`} {
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
	if built, _ := os.ReadFile(filepath.Join(dir, "docs", "docs.built.json")); !strings.Contains(string(built), "\\u003epages/pages.go\\u003c/figcaption") {
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
	var pages []struct {
		Slug, Title, Markdown, Source string
		Sections                      json.RawMessage
	}
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
		if !strings.Contains(string(api.Sections), `id=\"`+id+`\"`) {
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

// The code charter's own docs show is code that exists: every line of a sample typed into a page (one that does
// not name its file, which Samples holds to the file) is a line of the example, of the site or of charter's
// screens, where it is compiled and tested. A sample that was true when it was written and is not any more fails
// here, with the line. A line that only names a file (// main.go) or stands for what is left out (…) is not code.
func TestCharterDocsShowRealCode(t *testing.T) {
	root := filepath.Join("..", "..")
	real := map[string]bool{}
	for _, dir := range []string{"example", "site", filepath.Join("app", "screens")} {
		filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "build" || name == "dist" || name == "sdk" || name == "docs" {
					return filepath.SkipDir
				}
				return nil
			}
			if !slices.Contains([]string{".go", ".gsx", ".sql", ".tmpl"}, filepath.Ext(name)) || strings.HasSuffix(name, ".x.go") {
				return nil
			}
			b, _ := os.ReadFile(path)
			for _, line := range strings.Split(string(b), "\n") {
				real[strings.Join(strings.Fields(line), " ")] = true
			}
			return nil
		})
	}
	pages, _ := filepath.Glob(filepath.Join(root, "site", "docs", "*.md"))
	if len(pages) == 0 || len(real) < 1000 {
		t.Fatalf("%d pages and %d lines of code: this is not charter's repository", len(pages), len(real))
	}
	names := regexp.MustCompile(`^// [\w./-]+\.(go|gsx|sql|jsonc)$`)
	for _, page := range pages {
		b, _ := os.ReadFile(page)
		typed := false
		for i, line := range strings.Split(string(b), "\n") {
			switch {
			case strings.HasPrefix(line, "```"):
				typed = slices.Contains([]string{"```go", "```gsx", "```sql"}, line)
				continue
			case !typed:
				continue
			}
			code := strings.Join(strings.Fields(line), " ") // gofmt lines things up by what is around them
			if code == "" || code == "…" || names.MatchString(code) || real[code] {
				continue
			}
			t.Errorf("%s:%d shows code that is nowhere in the example, the site or the screens:\n\t%s", filepath.Base(page), i+1, code)
		}
	}
}

// The groups of the docs, and the order they are read in, are the front page's: a line that is only bold names a
// group, and the pages the list under it links to are in it, in that order, whatever their files are numbered. A
// page it lists nowhere comes after, in no group, and what charter writes of the app is the Reference.
func TestGroups(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/index.md", "# An app\n\nWhat it is for. See [the start](01-start.md) too.\n\n**Write**\n\n- [Pages](03-pages.md), and [the API](02-api.md#operations).\n\n**Start**\n\n- [Starting](01-start.md): first.\n\nElsewhere: [a site](https://example.org/x.md).\n")
	write("docs/01-start.md", "# Starting\n\nFirst.\n")
	write("docs/02-api.md", "# The API\n\nSecond.\n\n## Operations\n")
	write("docs/03-pages.md", "# Pages\n\nThird.\n")
	write("docs/04-other.md", "# Other\n\nNot listed.\n")
	write("wrangler.jsonc", `{"name": "x", "vars": {"APP_NAME": "x"}}`)
	if _, err := Write(dir); err != nil {
		t.Fatal(err)
	}
	built, _ := os.ReadFile(filepath.Join(dir, "docs", "docs.built.json"))
	var pages []struct{ Slug, Group string }
	if err := json.Unmarshal(built, &pages); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range pages {
		got = append(got, p.Slug+"="+p.Group)
	}
	if want := "=, pages=Write, api=Write, start=Start, other=, reference-bindings=Reference"; strings.Join(got, ", ") != want {
		t.Errorf("the pages and their groups:\n%s\nwant:\n%s", strings.Join(got, ", "), want)
	}
}

// A page is laid out as gsxui's own doc pages are: a section from each heading of the second level, and a table as
// data, for gsxui's Table. A list and a link have the classes gsxui's doc pages give them.
func TestAPageIsSectionsAndTables(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "docs", "index.md"), []byte("# Start\n\nFirst.\n\n## One\n\n- a [link](https://example.com)\n\n| What | Where |\n|---|---|\n| a `page` | `pages/` |\n\nAfter.\n\n## Two\n\n1. first\n"), 0o644)
	if _, err := Write(dir); err != nil {
		t.Fatal(err)
	}
	var pages []struct{ Sections []section }
	b, _ := os.ReadFile(filepath.Join(dir, "docs", "docs.built.json"))
	if err := json.Unmarshal(b, &pages); err != nil || len(pages) != 1 {
		t.Fatal(err, len(pages))
	}
	s := pages[0].Sections
	if len(s) != 3 || len(s[1].Parts) != 3 {
		t.Fatalf("want 3 sections, the second of 3 parts (a list, a table, a paragraph): %+v", s)
	}
	table := s[1].Parts[1]
	if len(table.Head) != 2 || table.Head[0] != "What" || len(table.Rows) != 1 || table.Rows[0][0] != "a <code>page</code>" {
		t.Errorf("the table: %+v", table)
	}
	for part, want := range map[string]string{
		s[1].Parts[0].HTML:       `<ul class="` + ListClass + `">`,
		s[1].Parts[0].HTML + " ": `class="` + LinkClass + `"`,
		s[2].Parts[0].HTML:       `<ol class="` + OrderedClass + `">`,
	} {
		if !strings.Contains(part, want) {
			t.Errorf("no %s in %s", want, part)
		}
	}
}
