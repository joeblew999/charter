// Package setup is charter add locale:<id>: it sets a gsx app up for intl, from its go.mod's directory. Every step leaves
// alone what is already there, so it can run again (after gsxui init, say).
package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
)

// Options are what the app ships.
type Options struct {
	Locales  []string // BCP 47 ids, the source first
	Catalogs string   // the catalogs' directory, e.g. "locales"
	Gsxui    string   // where gsxui add vendors components ("" for none)
}

const (
	intlPkg    = "github.com/joeblew999/charter/intl"
	gsxintlPkg = "github.com/joeblew999/charter/intl" // intl.Value's renderer, the only gsx of intl
)

// Init writes charter.toml's [intl], the catalogs, gsx.toml's filters and renderers, and .gitignore's generated files
// into the app at root, logging each change.
func Init(root string, o Options, log io.Writer) error {
	if len(o.Locales) == 0 || o.Catalogs == "" {
		return errors.New("intl init: locales and catalogs are required")
	}
	write := func(name, content string) error {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		fmt.Fprintln(log, "wrote", name)
		return os.WriteFile(path, []byte(content), 0o644)
	}

	q := make([]string, len(o.Locales))
	for i, id := range o.Locales {
		q[i] = `"` + id + `"`
	}
	conf := "\n# The app's languages (charter run languages; go doc github.com/joeblew999/charter/intl/config): BCP 47 ids, the\n# source first.\n[intl]\n" +
		"locales = [" + strings.Join(q, ", ") + "]\n" +
		"# <locale>.toml per language, ICU MessageFormat; charter build writes the Go package of messages there.\n" +
		"catalogs = " + `"` + o.Catalogs + `"` + "\n"
	if o.Gsxui != "" {
		conf += "# Where gsxui add vendors components: their own text is translated from the catalogs' [gsxui].\n" +
			"gsxui = " + `"` + o.Gsxui + `"` + "\n"
	}
	conf += "\n# Guidance for whoever translates, people or Claude (charter run todo shows it); \"*\" is for every language.\n" +
		"[intl.notes]\n\"*\" = \"\"\n"
	path := filepath.Join(root, "charter.toml")
	have, _ := os.ReadFile(path)
	if !regexp.MustCompile(`(?m)^\[intl\]`).Match(have) {
		if err := os.WriteFile(path, append(have, conf...), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(log, "wrote charter.toml's [intl]")
	}
	source := o.Locales[0]
	if err := write(filepath.Join(o.Catalogs, source+".toml"),
		"# The source catalog: every message the app shows, in ICU MessageFormat. Translations are <locale>.toml beside\n"+
			"# it, with the same tables and keys. A comment above a key or a table tells translators where the text appears.\n"+
			"# For example, written { locales.HomeTitle } and { locales.HomeNotes(len(notes)) } in markup:\n#\n"+
			"#   [home]\n#   # The page's heading.\n#   title = \"Welcome\"\n"+
			"#   # How many notes there are.\n#   notes = \"{n, plural, one {# note} other {# notes}}\"\n"); err != nil {
		return err
	}
	for _, id := range o.Locales[1:] {
		if err := write(filepath.Join(o.Catalogs, id+".toml"),
			"# "+id+": the translations of "+source+".toml, same tables and keys. charter run todo lists what is left.\n"); err != nil {
			return err
		}
	}

	// A gsx app gets gsx.toml's filters and renderers (intl); an app without gsx gets nothing of gsx's.
	if UsesGsx(root, o.Gsxui) {
		renderers := [][2]string{
			{intlPkg + ".Msg", intlPkg + ".Translate"},
			{intlPkg + ".Value", gsxintlPkg + ".Render"},
			{intlPkg + ".Plain", intlPkg + ".PlainText"},
		}
		if err := gsxToml(filepath.Join(root, "gsx.toml"), renderers, log); err != nil {
			return err
		}
		if err := GsxuiRenderer(root, o.Gsxui, log); err != nil {
			return err
		}
	}
	return gitignore(filepath.Join(root, ".gitignore"), log)
}

// UsesGsx reports whether the app at root is a gsx app: it has gsxui (gsxui), a gsx.toml, or .gsx files of its own.
// Its go.mod isn't the sign: the module an app is a folder of may require gsx for packages of its own (the template,
// where it is, is a folder of charter run languages's, which needs gsx).
func UsesGsx(root, gsxui string) bool {
	if gsxui != "" {
		return true
	}
	if _, err := os.Stat(filepath.Join(root, "gsx.toml")); err == nil {
		return true
	}
	found := errors.New("found")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir // a module of its own
			}
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".gsx") {
			return found
		}
		return nil
	})
	return err == found
}

// GoMod is the go.mod of the module root is in, the nearest up from it: the app's own, or for the template where it is,
// charter run languages's. "" when root is in none. It reads the files, so it needs no Go.
func GoMod(root string) string {
	for dir := root; ; dir = filepath.Dir(dir) {
		if p := filepath.Join(dir, "go.mod"); fileExists(p) {
			return p
		}
		if filepath.Dir(dir) == dir {
			return ""
		}
	}
}

// DetectGsxui is where gsxui vendors components in the app at root (gsxui.json's ui), "" without gsxui.
func DetectGsxui(root string) string {
	var g struct{ UI string }
	if b, err := os.ReadFile(filepath.Join(root, "gsxui.json")); err == nil && json.Unmarshal(b, &g) == nil {
		return g.UI
	}
	return ""
}

var catalogsKey = regexp.MustCompile(`(?m)^catalogs\s*=.*\n`)

// addGsxui writes gsxui = dir into an charter.toml's [intl] set up before gsxui was: init run again after gsxui init.
// UI writes ui = dir into the app's charter.toml's [intl] when it has no ui: where charter add screen:<name> put intl's screens.
func UI(root, dir string, log io.Writer) error {
	return addKey(filepath.Join(root, "charter.toml"), "ui", dir,
		"Where charter add screen:<name> copies intl's screens (package intlui): the language list, the language offer, the time zone\n# and clock dialog.", log)
}

// addKey writes key = value, below its comment, after [intl] catalogs line, unless the file sets key already.
func addKey(path, key, value, comment string, log io.Writer) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(b)
	if regexp.MustCompile(`(?m)^` + key + `\s*=`).MatchString(s) {
		return nil
	}
	line := "# " + comment + "\n" + key + " = \"" + value + "\"\n"
	if loc := catalogsKey.FindStringIndex(s); loc != nil {
		s = s[:loc[1]] + line + s[loc[1]:]
	} else {
		s = line + s
	}
	fmt.Fprintf(log, "updated charter.toml's [intl]: %s = %s\n", key, value)
	return os.WriteFile(path, []byte(s), 0o644)
}

// GsxuiRenderer registers the renderer for the text gsxui components write themselves (i18n.T) in gsx.toml, once
// the app has gsxui's i18n package: gsxui add vendors it with the first component that writes text, and until then
// gsx would fail on a renderer for a package that isn't there. charter build calls it every time, so the renderer
// arrives with the package.
func GsxuiRenderer(root, gsxui string, log io.Writer) error {
	if gsxui == "" {
		return nil
	}
	dir := filepath.Join(root, gsxui, "i18n")
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	pkg, err := catalog.PackagePath(dir)
	if err != nil {
		return err
	}
	return gsxToml(filepath.Join(root, "gsx.toml"), [][2]string{{pkg + ".T", pkg + ".Translate"}}, log)
}

var (
	filterLine = regexp.MustCompile(`(?m)^filter_packages\s*=\s*\[`)
	tableLine  = regexp.MustCompile(`(?m)^\[`)
)

// header finds a TOML table's header line (a comment or \r after it allowed); the index is just past it.
func header(s, table string) int {
	loc := regexp.MustCompile(`(?m)^\[` + regexp.QuoteMeta(table) + `\][ \t]*(#[^\n]*)?\r?\n`).FindStringIndex(s)
	if loc == nil {
		return -1
	}
	return loc[1]
}

// gsxToml registers intl's filter package and renderers in gsx.toml (gsx Configuration guide), creating it if need be.
func gsxToml(path string, renderers [][2]string, log io.Writer) error {
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s, before := string(b), string(b)
	filters := gsxintlPkg + "/filters"
	if !strings.Contains(s, `"`+filters+`"`) {
		line := "# intl's gsx filters: { t |> date(opts) }, { n |> number }, href={ \"/about\" |> localize } (go doc github.com/joeblew999/charter/gsxintl/filters).\n"
		if loc := filterLine.FindStringIndex(s); loc != nil {
			s = s[:loc[1]] + `"` + filters + `", ` + s[loc[1]:]
		} else if loc := tableLine.FindStringIndex(s); loc != nil {
			s = s[:loc[0]] + line + `filter_packages = ["` + filters + `"]` + "\n\n" + s[loc[0]:]
		} else {
			s += line + `filter_packages = ["` + filters + `"]` + "\n"
		}
	}
	var add []string
	for _, r := range renderers {
		if !strings.Contains(s, `"`+r[0]+`"`) {
			add = append(add, `"`+r[0]+`" = "`+r[1]+`"`)
		}
	}
	if len(add) > 0 {
		block := strings.Join(add, "\n") + "\n"
		if at := header(s, "renderers"); at >= 0 {
			s = s[:at] + block + s[at:]
		} else {
			if s != "" && !strings.HasSuffix(s, "\n\n") {
				s = strings.TrimRight(s, "\n") + "\n\n"
			}
			s += "# intl: a message or an Intl call written in markup renders in the request's language.\n[renderers]\n" + block
		}
	}
	if s == before {
		return nil
	}
	fmt.Fprintln(log, "updated gsx.toml: intl's filters and renderers")
	return os.WriteFile(path, []byte(s), 0o644)
}

// addToTable puts lines at the start of TOML table name, adding the table at the end if there is none.
func addToTable(s, name, lines string) string {
	if at := header(s, name); at >= 0 {
		return s[:at] + lines + s[at:]
	}
	if s != "" && !strings.HasSuffix(s, "\n\n") {
		s = strings.TrimRight(s, "\n") + "\n\n"
	}
	return s + "[" + name + "]\n" + lines
}

// gitignore keeps charter build's output out of git (intl.sum stays in).
func gitignore(path string, log io.Writer) error {
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	s := string(b)
	var add []string
	for _, p := range []string{"intl_gen.go", "intl-messages.mjs", "intl-workerd/", "/xliff/"} {
		if !regexp.MustCompile(`(?m)^/?` + regexp.QuoteMeta(p) + `$`).MatchString(s) {
			add = append(add, p)
		}
	}
	if len(add) == 0 {
		return nil
	}
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	s += "# charter build output (charter run languages)\n" + strings.Join(add, "\n") + "\n"
	fmt.Fprintln(log, "updated .gitignore:", strings.Join(add, ", "))
	return os.WriteFile(path, []byte(s), 0o644)
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
