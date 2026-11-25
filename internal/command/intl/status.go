package intl

import (
	"fmt"
	"os"
)

// status is mise-intl status: per locale, translated, inherited, stale, missing; exit 1 if one is incomplete.
func status(args []string) {
	c := app()
	p := read(c)
	ok := true
	for _, l := range p.Locales {
		fmt.Println(l.Summary(len(p.Messages)))
		ok = ok && l.Current() && len(l.Obsolete) == 0
	}
	if !ok {
		fmt.Println("mise-intl todo lists what is left")
		os.Exit(1)
	}
}
