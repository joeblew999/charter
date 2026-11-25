// Package config reads an app's languages from its charter.toml: [intl], the languages it ships and where their
// catalogs are. charter intl and intltest find it by walking up from the working directory.
//
//	# charter.toml
//	[intl]
//	locales = ["en", "de", "ar"]   # BCP 47 ids, the source (default) first
//	catalogs = "locales"           # <locale>.toml (ICU MessageFormat), and the generated package
//	gsxui = "ui"                   # optional: where gsxui add vendors components; gen translates their i18n.T
//	ui = "pages/intl"              # optional: where charter intl add copies intl's screens (default pages/intl)
//	xliff = "xliff"                # optional: where charter intl export writes and import reads (default xliff)
//	badge = "docs/badges/intl.svg" # optional: an SVG badge gen keeps current, "languages | 14, all translated"
//
//	[intl.inherit]                 # optional: a locale that takes what it doesn't translate from another
//	en-IN = "en"                   # (charter intl check confirms with Intl that both write the same script)
//
//	[intl.translate]               # optional: charter intl translate (Claude Code)
//	model = "sonnet"               # the model it runs on (claude --model); default: Claude Code's
//	workers = 6                    # languages translated at once (default 6)
//
//	[intl.notes]                   # optional: guidance for whoever translates (people, and Claude: charter intl todo)
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

// File is the file the languages are in, as its [intl].
const File = "charter.toml"

// Config is charter.toml's [intl].
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
	Root string `toml:"-"` // the app's folder: charter.toml's
}

// Note is the translators' guidance for locale id: the notes for every locale ("*"), then its own.
func (c Config) Note(id string) string {
	return strings.TrimSpace(c.Notes["*"] + " " + c.Notes[id])
}

// XLIFFDir is where export writes and import reads XLIFF: [intl] xliff, else "xliff".
func (c Config) XLIFFDir() string {
	if c.XLIFF == "" {
		return filepath.Join(c.Root, "xliff")
	}
	return filepath.Join(c.Root, c.XLIFF)
}

// UIDir is where charter intl add copies intl's screens, relative to the app's root: [intl] ui, else pages/intl.
func (c Config) UIDir() string {
	if c.UI == "" {
		return filepath.Join("pages", "intl")
	}
	return filepath.FromSlash(c.UI)
}

// CatalogDir is the catalogs' directory.
func (c Config) CatalogDir() string { return filepath.Join(c.Root, c.Catalogs) }

// Find reads [intl] of the nearest charter.toml at or above dir.
func Find(dir string) (Config, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Config{}, err
	}
	for {
		path := filepath.Join(dir, File)
		if _, err := os.Stat(path); err == nil {
			var file struct {
				Intl *Config `toml:"intl"`
			}
			if _, err := toml.DecodeFile(path, &file); err != nil {
				return Config{}, err
			}
			if file.Intl == nil {
				return Config{}, errors.New(path + " has no [intl]: the app's languages")
			}
			c := *file.Intl
			if len(c.Locales) == 0 || c.Catalogs == "" {
				return Config{}, errors.New(path + ": [intl] locales and catalogs are required")
			}
			c.Root = dir
			return c, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, errors.New("no " + File + " with an [intl] at or above the working directory")
		}
		dir = parent
	}
}
