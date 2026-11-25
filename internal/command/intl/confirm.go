package intl

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
)

// confirm is mise-intl confirm [<locale>[:<key>]…]: stale translations that are still right, kept (every one
// of them by default).
func confirm(args []string) {
	c := app()
	// Under a lock: translate's workers confirm at the same time, and each rewrites intl.sum.
	must(locked(filepath.Join(c.CatalogDir(), "intl.sum"), func() error {
		p, err := catalog.Read(c)
		if err != nil {
			return err
		}
		if len(args) == 0 { // every stale translation: the source's change didn't change their meaning
			for _, l := range p.Locales[1:] {
				args = append(args, l.ID)
			}
		}
		keys := map[*catalog.Locale][]string{}
		for _, a := range args {
			id, key, _ := strings.Cut(a, ":")
			l := p.Locale(id)
			if l == nil || l == p.Locales[0] {
				return fmt.Errorf("intl: %s is not a shipped translation locale", id)
			}
			if key != "" {
				keys[l] = append(keys[l], key)
			} else {
				keys[l] = keys[l][:0:0]
			}
		}
		for l, ks := range keys {
			n, err := p.Confirm(l, ks...)
			if err != nil {
				return err
			}
			fmt.Printf("%s: %d confirmed\n", l.ID, n)
		}
		return nil
	}))
}
