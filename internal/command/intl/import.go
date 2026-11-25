package intl

import (
	"fmt"
	"os"
	"path/filepath"
)

// importXLIFF is mise-intl import [file.xlf…]: translators' XLIFF read back into the catalogs, all of
// xliff/in by default. import is Go's keyword, so the function isn't named for its command.
func importXLIFF(args []string) {
	c := app()
	if len(args) == 0 { // every file translators sent back
		in := filepath.Join(c.XLIFFDir(), "in")
		files, err := filepath.Glob(filepath.Join(in, "*.xlf"))
		must(err)
		if len(files) == 0 {
			fail(fmt.Errorf("intl: no .xlf files in %s: put translators' files there", in))
		}
		args = files
	}
	for _, file := range args {
		b, err := os.ReadFile(file)
		must(err)
		p := read(c)
		l, changed, confirmed, err := p.ImportXLIFF(b)
		must(err)
		fmt.Printf("%s → %s: %d changed, %d confirmed\n", file, l.File, changed, confirmed)
	}
	must(read(c).Generate(os.Stderr))
}
