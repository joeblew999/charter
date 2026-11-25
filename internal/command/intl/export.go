package intl

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/site/intl/intltest"
)

// export is charter intl export [--all] [--out dir] [locale…]: XLIFF 2.0 for translators, what is left (or
// everything), into xliff/out (intl.toml's xliff).
func export(args []string) {
	c := app()
	f := flag.NewFlagSet("export", flag.ExitOnError)
	all := f.Bool("all", false, "every message, not only what is left to translate")
	out := f.String("out", filepath.Join(c.XLIFFDir(), "out"), "directory for <locale>.xlf")
	f.Parse(args)
	p := read(c)
	startIntl()
	must(os.MkdirAll(*out, 0o755))
	for _, l := range pick(p, f.Args()) {
		cardinal, ordinal, err := intltest.Categories(l.ID)
		must(err)
		note := fmt.Sprintf("%s plural cases: %s; selectordinal cases: %s.", l.ID, strings.Join(cardinal, ", "), strings.Join(ordinal, ", "))
		b, n, err := p.ExportXLIFF(l, *all, note)
		must(err)
		path := filepath.Join(*out, l.ID+".xlf")
		must(os.WriteFile(path, b, 0o644))
		fmt.Printf("%s: %d messages\n", path, n)
	}
}
