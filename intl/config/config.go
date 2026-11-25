// Package config reads an app's intl.toml: the languages it ships and where their catalogs are. mise-intl and
// intltest find it by walking up from the working directory.
//
//	# intl.toml
//	locales = ["en", "de", "ar"]   # BCP 47 ids, the source (default) first
//	catalogs = "locales"           # <locale>.toml (ICU MessageFormat), and the generated package
//	gsxui = "ui"                   # optional: where gsxui add vendors components; gen translates their i18n.T
//	ui = "views/intl"              # optional: where mise-intl add copies intl's screens (default views/intl)
//	xliff = "xliff"                # optional: where mise-intl export writes and import reads (default xliff)
//	badge = "docs/badges/intl.svg" # optional: an SVG badge gen keeps current, "languages | 14, all translated"
//
//	[inherit]                      # optional: a locale that takes what it doesn't translate from another
//	en-IN = "en"                   # (mise-intl check confirms with Intl that both write the same script)
//
//	[translate]                    # optional: mise-intl translate (Claude Code)
//	model = "sonnet"               # the model it runs on (claude --model); default: Claude Code's
//	workers = 6                    # languages translated at once (default 6)
//
//	[notes]                        # optional: guidance for whoever translates (people, and Claude: mise-intl todo)
//	"*" = "A developer tool. Keep product names in English."
//	de = "Address the reader as du."
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// File is the config file's name.
const File = "intl.toml"

// Config is an app's intl.toml.
type Config struct {
	Locales   []string          `toml:"locales"`
	Catalogs  string            `toml:"catalogs"`
	Gsxui     string            `toml:"gsxui"`
	UI        string            `toml:"ui"`
	XLIFF     string            `toml:"xliff"`
	Badge     string            `toml:"badge"`
	Notes     map[string]string `toml:"notes"`
	Inherit   map[string]string `toml:"inherit"`
	Translate struct {
		Model   string `toml:"model"`
		Workers int    `toml:"workers"`
	} `toml:"translate"`
	Root string `toml:"-"` // the directory intl.toml is in
}

// Note is the translators' guidance for locale id: the notes for every locale ("*"), then its own.
func (c Config) Note(id string) string {
	return strings.TrimSpace(c.Notes["*"] + " " + c.Notes[id])
}

// XLIFFDir is where export writes and import reads XLIFF: intl.toml's xliff, else "xliff".
func (c Config) XLIFFDir() string {
	if c.XLIFF == "" {
		return filepath.Join(c.Root, "xliff")
	}
	return filepath.Join(c.Root, c.XLIFF)
}

// UIDir is where mise-intl add copies intl's screens, relative to the app's root: intl.toml's ui, else views/intl.
func (c Config) UIDir() string {
	if c.UI == "" {
		return filepath.Join("views", "intl")
	}
	return filepath.FromSlash(c.UI)
}

// CatalogDir is the catalogs' directory.
func (c Config) CatalogDir() string { return filepath.Join(c.Root, c.Catalogs) }

// Find reads the nearest intl.toml at or above dir.
func Find(dir string) (Config, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Config{}, err
	}
	for {
		path := filepath.Join(dir, File)
		if _, err := os.Stat(path); err == nil {
			var c Config
			if _, err := toml.DecodeFile(path, &c); err != nil {
				return Config{}, err
			}
			if len(c.Locales) == 0 || c.Catalogs == "" {
				return Config{}, errors.New(path + ": locales and catalogs are required")
			}
			c.Root = dir
			return c, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, errors.New("no " + File + " at or above the working directory")
		}
		dir = parent
	}
}
