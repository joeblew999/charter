// Package registry is a registry of gsx components that an app copies in and then owns, the way gsxui add copies
// gsxui's (https://ui.gsxhq.dev): each component a gsx template made of gsxui components, with its text already
// translated and, for one that runs in the browser, the files it loads there. charter add screen:<name> copies
// charter's (app/screens). [Registry.Add] is that command.
package registry

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/joeblew999/charter/internal/intlkit/catalog"
)

// Registry is a set of components and everything add copies with them.
type Registry struct {
	// Command is what adds them, for messages: "charter add screen:<name>".
	Command string
	// Table is the catalogs' table the components' messages are under: "intl" ([intl]), "gsxkit" ([gsxkit.map], …).
	Table string
	// Files holds <component>.gsx.tmpl (gsx generate leaves a .tmpl alone, so the registry's own module never builds
	// them), locales/<id>.toml (their text under Table, and the text gsxui's own components write under [gsxui]) and
	// assets/<file>, what a component loads in the browser.
	Files fs.FS
	// Locales is the templates' import of their text, which Source points at the app's catalogs.
	Locales string
	// Components are the registry's components.
	Components []Component
}

// Component is one of a registry's components.
type Component struct {
	Name   string   // what add takes, and the file's name in the app: language-list.gsx
	Gsxui  []string // the gsxui components it is made of, as gsxui add names them
	Needs  []string // the registry's other components it writes: add copies them with it
	Schema string   // tables it keeps what it knows in: add writes them into db/schema.sql, where they are not yet
	Assets []string // the files under assets/ it loads in the browser: add copies them into the app's assets folder
	Use    string   // where it goes in a page, once added: add prints it
}

// Find is the component called name.
func (r *Registry) Find(name string) (Component, bool) {
	i := slices.IndexFunc(r.Components, func(c Component) bool { return c.Name == name })
	if i < 0 {
		return Component{}, false
	}
	return r.Components[i], true
}

// Names are the components' names, for usage messages.
func (r *Registry) Names() []string {
	var out []string
	for _, c := range r.Components {
		out = append(out, c.Name)
	}
	return out
}

// Paths are the app's packages the components import.
type Paths struct {
	Catalogs string // the catalogs' Go package: "example.com/app/locales"
	Gsxui    string // the directory gsxui add vendors into: "example.com/app/ui"
}

// gsxuiImport is gsxui's own path for its components in a template: Source points it where the app's gsxui add
// vendored them, as gsxui add rewrites it.
const gsxuiImport = "github.com/gsxhq/gsxui/ui"

// Source is the component's file for an app: its imports point at the app's catalogs and gsxui components, sorted as
// gsx fmt sorts them.
func (r *Registry) Source(c Component, p Paths) ([]byte, error) {
	b, err := fs.ReadFile(r.Files, c.Name+".gsx.tmpl")
	if err != nil {
		return nil, err
	}
	start := bytes.Index(b, []byte("import (\n"))
	end := bytes.Index(b, []byte("\n)\n"))
	if start < 0 || end < start {
		return nil, fmt.Errorf("%s.gsx.tmpl: no import block", c.Name)
	}
	start += len("import (\n")
	var imports []string
	for _, line := range strings.Split(string(b[start:end]), "\n") {
		spec := strings.TrimSpace(line)
		imp, err := strconv.Unquote(spec)
		if err != nil {
			return nil, fmt.Errorf("%s.gsx.tmpl: import %s", c.Name, spec)
		}
		switch {
		case imp == r.Locales:
			spec = strconv.Quote(p.Catalogs)
			if path.Base(p.Catalogs) != "locales" {
				spec = "locales " + spec
			}
		case imp == gsxuiImport || strings.HasPrefix(imp, gsxuiImport+"/"):
			spec = strconv.Quote(p.Gsxui + strings.TrimPrefix(imp, gsxuiImport))
		}
		imports = append(imports, spec)
	}
	slices.SortFunc(imports, func(a, b string) int { return strings.Compare(importPath(a), importPath(b)) })
	var out bytes.Buffer
	out.Write(b[:start])
	for _, spec := range imports {
		out.WriteString("\t" + spec + "\n")
	}
	out.Write(b[end+1:])
	return out.Bytes(), nil
}

// Format is src as the app's own gsx fmt writes the file at rel (from the app's root, which is dir). Add writes every
// copy formatted, and the app's checks compare its copies with Format(Source(…)): a gsx release that formats
// differently changes both, so a template and its copy never drift.
func Format(dir, rel string, src []byte) ([]byte, error) {
	cmd := exec.Command("go", "tool", "gsx", "fmt", "-stdin-filename", rel)
	var out, stderr bytes.Buffer
	cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = dir, bytes.NewReader(src), &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gsx fmt %s: %v\n%s", rel, err, stderr.String())
	}
	return out.Bytes(), nil
}

func importPath(spec string) string {
	_, p, _ := strings.Cut(spec, `"`)
	return p
}

// Package is the Go package the components' files declare: "screens".
func (r *Registry) Package() (string, error) {
	if len(r.Components) == 0 {
		return "", fmt.Errorf("%s: no components", r.Command)
	}
	b, err := fs.ReadFile(r.Files, r.Components[0].Name+".gsx.tmpl")
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(string(b), "\n")
	name, ok := strings.CutPrefix(first, "package ")
	if !ok {
		return "", fmt.Errorf("%s.gsx.tmpl: no package clause on its first line", r.Components[0].Name)
	}
	return strings.TrimSpace(name), nil
}

// Asset is the file under assets/ called name.
func (r *Registry) Asset(name string) ([]byte, error) { return fs.ReadFile(r.Files, "assets/"+name) }

// Keys are the messages the component writes (keys under Table), in the source catalog's order.
func (r *Registry) Keys(c Component) ([]string, error) {
	b, err := fs.ReadFile(r.Files, c.Name+".gsx.tmpl")
	if err != nil {
		return nil, err
	}
	uses := regexp.MustCompile(`\blocales\.(` + catalog.GoName(r.Table) + `[A-Za-z0-9]*)`)
	names := map[string]bool{}
	for _, m := range uses.FindAllSubmatch(b, -1) {
		names[string(m[1])] = true
	}
	src, err := r.Notes()
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, k := range src.Keys {
		if strings.HasPrefix(k, r.Table+".") && names[catalog.GoName(k)] {
			keys = append(keys, k)
			delete(names, catalog.GoName(k))
		}
	}
	if len(names) > 0 {
		return nil, fmt.Errorf("%s.gsx.tmpl writes messages locales/en.toml doesn't have: %v", c.Name, slices.Sorted(maps.Keys(names)))
	}
	return keys, nil
}

// Languages are the languages the registry ships its text in, the source (en) first.
func (r *Registry) Languages() []string {
	entries, _ := fs.ReadDir(r.Files, "locales")
	out := []string{"en"}
	for _, e := range entries {
		if id := strings.TrimSuffix(e.Name(), ".toml"); id != "en" {
			out = append(out, id)
		}
	}
	return out
}

// Text is the components' text in locale id, by key (Table's keys and [gsxui] ones) in ICU MessageFormat: nil when the
// registry doesn't ship that language.
func (r *Registry) Text(id string) (map[string]string, error) {
	b, err := fs.ReadFile(r.Files, "locales/"+id+".toml")
	if err != nil {
		return nil, nil
	}
	var tables map[string]any
	if err := toml.Unmarshal(b, &tables); err != nil {
		return nil, fmt.Errorf("locales/%s.toml: %w", id, err)
	}
	out := map[string]string{}
	var walk func(prefix string, t map[string]any) error
	walk = func(prefix string, t map[string]any) error {
		for k, v := range t {
			switch v := v.(type) {
			case string:
				out[prefix+k] = v
			case map[string]any:
				if err := walk(prefix+k+".", v); err != nil {
					return err
				}
			default:
				return fmt.Errorf("locales/%s.toml: %s%s is neither a message nor a table", id, prefix, k)
			}
		}
		return nil
	}
	if err := walk("", tables); err != nil {
		return nil, err
	}
	return out, nil
}

// SourceNotes is the source catalog's layout: its keys in order, and the comment lines above each key and each table
// header, which add writes above them in the app's source catalog for its translators.
type SourceNotes struct {
	Keys   []string
	Notes  map[string][]string // key → its comment lines
	Tables map[string][]string // table → the comment lines above its header
}

// Notes is the source catalog's layout and comments.
func (r *Registry) Notes() (SourceNotes, error) {
	b, err := fs.ReadFile(r.Files, "locales/en.toml")
	if err != nil {
		return SourceNotes{}, err
	}
	s := SourceNotes{Notes: map[string][]string{}, Tables: map[string][]string{}}
	var table string
	var block []string
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			block = nil
		case t[0] == '#':
			block = append(block, strings.TrimSpace(strings.TrimPrefix(t, "#")))
		case t[0] == '[':
			table = strings.Trim(t, "[]")
			s.Tables[table], block = block, nil
		default:
			name, _, _ := strings.Cut(t, "=")
			key := table + "." + strings.TrimSpace(name)
			s.Keys = append(s.Keys, key)
			s.Notes[key], block = block, nil
		}
	}
	return s, nil
}
