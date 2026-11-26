package docsgen

import (
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
	b, _ := os.ReadFile(filepath.Join(dir, "docs", "docs.built.go"))
	code := string(b)
	for _, want := range []string{`Slug: "", Title: "Start"`, `Slug: "pages", Title: "Pages"`, `Description: "How pages work."`, `Description: "What it is. See the pages."`, `{ID: "routes", Title: "Routes", Depth: 2}`,
		`href=\"pages#routes\"`, `href=\"./\"`, `<h2 id=\"routes\">Routes</h2>`, `<pre class=\"chroma\">`, `<span class=\"mi\">1</span>`} {
		if !strings.Contains(code, want) {
			t.Errorf("no %s in:\n%s", want, code)
		}
	}
	if strings.Index(code, `Title: "Start"`) > strings.Index(code, `Title: "Pages"`) {
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
	if built, _ := os.ReadFile(filepath.Join(dir, "docs", "docs.built.go")); !strings.Contains(string(built), "<figcaption>pages/pages.go</figcaption>") {
		t.Errorf("the page does not say which file the code is from:\n%s", built)
	}
	os.WriteFile(page, []byte("# Start\n\n```go file=pages/pages.go#Gone\n```\n"), 0o644)
	if _, err := Samples(dir, false); err == nil {
		t.Error("a sample of a declaration that is not there passed")
	}
}
