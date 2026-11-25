package intl

import (
	"fmt"
	"path/filepath"
)

// catalogs is mise-intl catalogs: the catalogs' Go package name (locales), for scripts.
func catalogs(args []string) {
	c := app()
	fmt.Println(filepath.Base(c.CatalogDir()))
}
