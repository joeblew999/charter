package intl

import (
	"fmt"
	"path/filepath"
)

// catalogs is charter intl catalogs: the catalogs' Go package name (locales), for scripts.
func catalogs(args []string) {
	c := app()
	fmt.Println(filepath.Base(c.CatalogDir()))
}
