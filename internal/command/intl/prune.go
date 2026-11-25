package intl

import (
	"fmt"
	"os"
	"strings"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
)

// prune is charter intl prune: the keys the source no longer has, removed from the catalogs.
func prune(args []string) {
	c := app()
	p := read(c)
	n := 0
	for _, l := range p.Locales[1:] {
		if len(l.Obsolete) > 0 {
			must(catalog.RemoveMessages(l.File, l.Obsolete))
			fmt.Printf("%s: removed %s\n", l.File, strings.Join(l.Obsolete, ", "))
			n += len(l.Obsolete)
		}
	}
	if n == 0 {
		fmt.Println("nothing to prune")
	}
	must(read(c).Generate(os.Stderr))
}
