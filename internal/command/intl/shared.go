package intl

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
	"github.com/joeblew999/charter/intl/config"
	"github.com/joeblew999/charter/intl/workerd"
)

// writeChanged writes b to path unless it holds b already, and names what it wrote relative to root.
func writeChanged(root, path string, b []byte) (bool, error) {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return false, err
	}
	if rel, err := filepath.Rel(root, path); err == nil {
		path = rel
	}
	fmt.Println("wrote", path)
	return true, nil
}

func read(c config.Config) *catalog.Project {
	p, err := catalog.Read(c)
	must(err)
	return p
}

// pick is the locales named (every translation locale when none is), never the source.
func pick(p *catalog.Project, ids []string) []*catalog.Locale {
	if len(ids) == 0 {
		return p.Locales[1:]
	}
	var out []*catalog.Locale
	for _, id := range ids {
		l := p.Locale(id)
		if l == nil || l == p.Locales[0] {
			fail(fmt.Errorf("intl: %s is not a shipped translation locale", id))
		}
		out = append(out, l)
	}
	return out
}

// startIntl sets intl's Intl: workerd (workerd.Start), which stays for every command after. In a checkout that has
// not been built yet, the files workerd serves are written first, so the first command a developer runs works.
func startIntl() {
	c := app()
	if _, err := os.Stat(filepath.Join(c.CatalogDir(), "intl-workerd", "intl-workerd.capnp")); err != nil {
		must(read(c).Generate(os.Stderr))
	}
	if err := workerd.Start(); err != nil {
		fail(fmt.Errorf("intl: Intl needs workerd: %w", err))
	}
}

func must(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// exists reports whether path is there.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
