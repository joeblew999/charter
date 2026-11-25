package catalog

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Unused are the source messages nothing uses: no Go or gsx file of the app's module names its constructor
// (<catalogs>.<GoName>, e.g. locales.BoardNotes) and no gsxui component writes its text. Generated files and other
// modules nested in the app are not looked at.
func (p *Project) Unused() ([]string, error) {
	pkg := filepath.Base(p.Config.CatalogDir())
	ref := regexp.MustCompile(`\b` + regexp.QuoteMeta(pkg) + `\.([A-Z][A-Za-z0-9_]*)\b`)
	used := map[string]bool{}
	root := p.Config.Root
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "testdata" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, ".gsx") || strings.HasSuffix(name, ".x.go") || name == "intl_gen.go" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range ref.FindAllSubmatch(b, -1) {
			used[string(m[1])] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	texts, err := p.Gsxui()
	if err != nil {
		return nil, err
	}
	for _, t := range texts {
		used[GoName(t.Key)] = true
	}
	var out []string
	for _, m := range p.Messages {
		if !used[GoName(m.Key)] {
			out = append(out, m.Key)
		}
	}
	return out, nil
}
