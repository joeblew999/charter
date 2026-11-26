package intl

import (
	"errors"
	"flag"
	"os"
	"path/filepath"

	intljs "github.com/joeblew999/charter/internal/intlkit/js"
)

// assets is charter build [--worker dir] [--browser dir]: the JavaScript a Worker and a page import.
func assets(args []string) {
	c := app()
	f := flag.NewFlagSet("assets", flag.ExitOnError)
	worker := f.String("worker", "", "directory for the Worker's modules")
	browser := f.String("browser", "", "directory for the page's modules")
	f.Parse(args)
	if *worker == "" && *browser == "" {
		fail(errors.New("charter build: --worker <dir>, --browser <dir>, or both"))
	}
	if *worker != "" {
		must(copyJS(*worker, "intl.mjs", "intl-worker.mjs"))
		b, err := os.ReadFile(filepath.Join(c.BuiltDir(), "intl-messages.mjs"))
		must(err)
		must(os.WriteFile(filepath.Join(*worker, "intl-messages.mjs"), b, 0o644))
	}
	if *browser != "" {
		must(copyJS(*browser, "intl.mjs", "intl-browser.mjs"))
	}
}

func copyJS(dir string, names ...string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, n := range names {
		b, err := intljs.FS.ReadFile(n)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, n), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}
