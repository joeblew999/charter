package intl

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/site/intl/config"
)

// markup is charter intl markup: the app's own markup files, the .gsx and .go files beside its .gsx (not gsxui's, not
// nested modules), for gsx's checks.
func markup(args []string) {
	c := app()
	files, err := markupFiles(c)
	must(err)
	for _, f := range files {
		fmt.Println(f)
	}
}

// markup is the app's own markup, for gsx's checks: the .gsx files of every directory that has one, and the
// hand-written .go files beside them, relative to the app's root. Not what gsxui vendored, not nested modules, and
// never a directory, so a check on these files can't reach into either.
func markupFiles(c config.Config) ([]string, error) {
	var out []string
	gsxui := filepath.Join(c.Root, c.Gsxui)
	err := filepath.WalkDir(c.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != c.Root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "testdata" ||
			c.Gsxui != "" && path == gsxui) {
			return filepath.SkipDir
		}
		if path != c.Root {
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			}
		}
		gsx, _ := filepath.Glob(filepath.Join(path, "*.gsx"))
		if len(gsx) == 0 {
			return nil
		}
		gofiles, _ := filepath.Glob(filepath.Join(path, "*.go"))
		for _, f := range append(gsx, gofiles...) {
			if strings.HasSuffix(f, ".x.go") {
				continue
			}
			rel, err := filepath.Rel(c.Root, f)
			if err != nil {
				return err
			}
			out = append(out, rel)
		}
		return nil
	})
	return out, err
}
