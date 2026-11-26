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
		`href=\"pages#routes\"`, `href=\"./\"`, `<h2 id=\"routes\">Routes</h2>`, `language-go`} {
		if !strings.Contains(code, want) {
			t.Errorf("no %s in:\n%s", want, code)
		}
	}
	if strings.Index(code, `Title: "Start"`) > strings.Index(code, `Title: "Pages"`) {
		t.Error("index.md is not first")
	}
}
