// Package config reads an app's languages from its charter.toml: [intl], the languages it ships and where their
// catalogs are. charter intl and intltest find it by walking up from the working directory.
//
//	# charter.toml
//	[intl]
//	locales = ["en", "de", "ar"]   # BCP 47 ids, the source (default) first
//	catalogs = "locales"           # <locale>.toml (ICU MessageFormat), and the generated package
//	gsxui = "ui"                   # optional: where gsxui add vendors components; gen translates their i18n.T
//	ui = "pages/intl"              # optional: where charter add screen:<name> copies intl's screens (default pages/intl)
//	xliff = "xliff"                # optional: where charter run intl:export writes and import reads (default xliff)
//	badge = "docs/badges/intl.svg" # optional: an SVG badge gen keeps current, "languages | 14, all translated"
//
//	[intl.inherit]                 # optional: a locale that takes what it doesn't translate from another
//	en-IN = "en"                   # (charter check confirms with Intl that both write the same script)
//
//	[intl.translate]               # optional: charter run intl:translate (Claude Code)
//	model = "sonnet"               # the model it runs on (claude --model); default: Claude Code's
//	workers = 6                    # languages translated at once (default 6)
//
//	[intl.notes]                   # optional: guidance for whoever translates (people, and Claude: charter run intl:todo)
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

// UIDir is where charter add screen:<name> copies intl's screens, relative to the app's root: [intl] ui, else pages/intl.
func (c Config) UIDir() string {
	if c.UI == "" {
		return filepath.Join("pages", "intl")
	}
	return filepath.FromSlash(c.UI)
}

// CatalogDir is the catalogs' directory.
func (c Config) CatalogDir() string { return filepath.Join(c.Root, c.Catalogs) }

// BuiltDir is where what charter builds from the catalogs for the runtime goes: .charter/intl in the app's folder,
// with everything else a build writes and nobody edits or commits. (The Go package of messages, messages.built.go,
// stays with the catalogs: it is their package.)
func (c Config) BuiltDir() string { return filepath.Join(c.Root, ".charter", "intl") }

// Find is the languages of the app at or above dir (the folder with wrangler.jsonc or charter.toml), by convention:
// the catalogs are locales/<locale>.toml, the languages are the catalogs there (en first, the source), and gsxui's
// components are in ui/. charter.toml's [intl] says anything else, and what the folders cannot: notes for
// translators, a language that inherits another's.
func Find(dir string) (Config, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Config{}, err
	}
	for {
		_, errW := os.Stat(filepath.Join(dir, "wrangler.jsonc"))
		_, errC := os.Stat(filepath.Join(dir, File))
		if errW == nil || errC == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, errors.New("no app (wrangler.jsonc) at or above the working directory")
		}
		dir = parent
	}
	var file struct {
		Intl *Config `toml:"intl"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, File), &file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	c := Config{}
	if file.Intl != nil {
		c = *file.Intl
	}
	c.Root = dir
	if c.Catalogs == "" {
		c.Catalogs = "locales"
	}
	if len(c.Locales) == 0 {
		catalogs, _ := filepath.Glob(filepath.Join(c.CatalogDir(), "*.toml"))
		for _, f := range catalogs {
			if id := strings.TrimSuffix(filepath.Base(f), ".toml"); id == "en" {
				c.Locales = append([]string{id}, c.Locales...)
			} else {
				c.Locales = append(c.Locales, id)
			}
		}
	}
	if len(c.Locales) == 0 {
		return Config{}, errors.New(c.CatalogDir() + " has no catalogs (<locale>.toml): the app's languages")
	}
	if c.Gsxui == "" {
		if _, err := os.Stat(filepath.Join(dir, "ui", "i18n")); err == nil {
			c.Gsxui = "ui"
		}
	}
	return c, nil
}
