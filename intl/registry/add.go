package registry

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
	"github.com/joeblew999/charter/internal/intlkit/setup"
	"github.com/joeblew999/charter/intl/config"
)

// Options are where add writes and how.
type Options struct {
	Dir       string // where the components' gsx goes, from the app's root: "pages/screens"
	Assets    string // where their browser files go, from the app's root: "web/gsxkit"
	Diff      bool   // show how the registry's components differ from the app's copies, writing nothing
	Overwrite bool   // replace copies the app changed
}

// Add copies the components named into the app c is: it writes each into o.Dir with its imports pointing at the app's packages, formatted by
// the app's gsx fmt, and its browser files into o.Assets, runs gsxui add for the gsxui components they are made of that the app doesn't
// have yet, and writes their text, already translated, into the
// catalogs: Table's messages, and [gsxui] for what the app's gsxui components write and the catalogs lack. A copy the app changed since add wrote it
// is left alone: o.Diff shows the difference, o.Overwrite replaces it. Its result is the command's exit code: 1 when
// o.Diff found a difference or a changed copy was kept.
func (r *Registry) Add(c config.Config, names []string, o Options) (int, error) {
	var picked []Component
	names = slices.Clone(names)
	for i := 0; i < len(names); i++ {
		name := names[i]
		comp, ok := r.Find(name)
		if !ok {
			return 1, fmt.Errorf("%s: no component %q; there are %s", r.Command, name, strings.Join(r.Names(), ", "))
		}
		if !slices.ContainsFunc(picked, func(o Component) bool { return o.Name == name }) {
			picked = append(picked, comp)
			names = append(names, comp.Needs...) // what it writes of the registry's own comes with it
		}
	}
	gsxui := cmp.Or(c.Gsxui, setup.DetectGsxui(c.Root))
	if gsxui == "" {
		return 1, errors.New("these components are made of gsxui components: set gsxui up first (gsxui init; https://ui.gsxhq.dev)")
	}
	catalogs, err := catalog.PackagePath(c.CatalogDir())
	if err != nil {
		return 1, err
	}
	components, err := catalog.PackagePath(filepath.Join(c.Root, gsxui))
	if err != nil {
		return 1, err
	}
	paths := Paths{Catalogs: catalogs, Gsxui: components}

	p, err := catalog.Read(c)
	if err != nil {
		return 1, err
	}
	// Every file a component brings: its gsx, then its browser files.
	type file struct {
		rel  string // from the app's root
		src  []byte
		text bool // a diff shows its lines; a bundle only that it differs
	}
	var files []file
	for _, comp := range picked {
		src, err := r.Source(comp, paths)
		if err != nil {
			return 1, err
		}
		rel := filepath.ToSlash(filepath.Join(o.Dir, comp.Name+".gsx"))
		if src, err = Format(c.Root, rel, src); err != nil {
			return 1, err
		}
		files = append(files, file{rel, src, true})
		for _, a := range comp.Assets {
			if o.Assets == "" {
				return 1, fmt.Errorf("%s has browser files: say where they go", comp.Name)
			}
			b, err := r.Asset(a)
			if err != nil {
				return 1, err
			}
			files = append(files, file{filepath.ToSlash(filepath.Join(o.Assets, a)), b, false})
		}
	}
	copied, differ := map[string]string{}, 0
	var kept []string
	for _, f := range files {
		path := filepath.Join(c.Root, f.rel)
		old, err := os.ReadFile(path)
		had := err == nil
		switch {
		case o.Diff:
			if !bytes.Equal(old, f.src) {
				differ++
				if f.text {
					fmt.Print(unified(f.rel, old, f.src))
				} else {
					fmt.Printf("%s differs\n", f.rel)
				}
			}
			continue
		case had && bytes.Equal(old, f.src):
		case had && !o.Overwrite && catalog.FileHash(old) != p.Copied(f.rel):
			kept = append(kept, f.rel)
			continue
		default:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return 1, err
			}
			if err := os.WriteFile(path, f.src, 0o644); err != nil {
				return 1, err
			}
			fmt.Println(map[bool]string{false: "added", true: "updated"}[had], f.rel)
		}
		copied[f.rel] = catalog.FileHash(f.src)
	}
	if o.Diff {
		if differ > 0 {
			return 1, nil
		}
		fmt.Printf("%s's components and the app's copies are the same\n", r.Command)
		return 0, nil
	}
	// The gsxui components they are made of, after the copies are written: gsxui add generates the app's Go, and a
	// copy written for an older release of this library may no longer compile against this one, which would stop it
	// before the copy that does was there.
	if err := gsxuiAdd(c.Root, gsxui, picked); err != nil {
		return 1, err
	}
	// The tables a component keeps what it knows in go where every table is written down: db/schema.sql. charter
	// writes the migration from there.
	for _, comp := range picked {
		if comp.Schema == "" {
			continue
		}
		schema := filepath.Join(c.Root, "db", "schema.sql")
		old, err := os.ReadFile(schema)
		if err != nil {
			return 1, fmt.Errorf("%s keeps what it knows in a database: the app needs db/schema.sql first", comp.Name)
		}
		first, _, _ := strings.Cut(comp.Schema[strings.Index(comp.Schema, "CREATE TABLE"):], "(")
		if bytes.Contains(old, []byte(strings.TrimSpace(first))) {
			continue
		}
		if err := os.WriteFile(schema, append(append(bytes.TrimRight(old, "\n"), "\n\n"...), comp.Schema...), 0o644); err != nil {
			return 1, err
		}
		fmt.Println("added its tables to db/schema.sql (charter check --fix writes the migration)")
	}
	texts, err := r.addText(c, p, picked)
	if err != nil {
		return 1, err
	}
	if p, err = catalog.Read(c); err != nil {
		return 1, err
	}
	for rel, h := range copied {
		p.SetCopied(rel, h)
	}
	for key, h := range texts {
		p.SetCopied(textRecord+key, h)
	}
	if err := p.Generate(os.Stderr); err != nil {
		return 1, err
	}
	if err := setup.GsxuiRenderer(c.Root, gsxui, os.Stderr); err != nil {
		return 1, err
	}
	gsx := exec.Command("go", "tool", "gsx", "generate", "-q")
	gsx.Dir, gsx.Stdout, gsx.Stderr = c.Root, os.Stdout, os.Stderr
	if err := gsx.Run(); err != nil {
		return 1, err
	}
	pkg, err := catalog.PackagePath(filepath.Join(c.Root, o.Dir))
	if err != nil {
		return 1, err
	}
	name, err := r.Package()
	if err != nil {
		return 1, err
	}
	fmt.Printf("\nimport %s %q, then:\n", name, pkg)
	for _, comp := range picked {
		fmt.Println(comp.Use)
	}
	if len(kept) > 0 {
		fmt.Fprintf(os.Stderr, "kept, since the app changed them: %s\n  %s -diff %s shows how; -overwrite replaces them\n",
			strings.Join(kept, ", "), r.Command, strings.Join(names, " "))
		return 1, nil
	}
	return 0, nil
}

// gsxuiAdd runs gsxui add for the gsxui components the components are made of that the app doesn't have.
func gsxuiAdd(root, dir string, comps []Component) error {
	var missing []string
	for _, c := range comps {
		for _, g := range c.Gsxui {
			// A component is a file (button.gsx) or a folder of them (icon/).
			_, file := os.Stat(filepath.Join(root, dir, g+".gsx"))
			_, folder := os.Stat(filepath.Join(root, dir, g))
			if file != nil && folder != nil && !slices.Contains(missing, g) {
				missing = append(missing, g)
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	// gsxui is the app's own tool (go.mod), at the version the app pins.
	cmd := exec.Command("go", append([]string{"tool", "gsxui", "add"}, missing...)...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = root, os.Stdout, os.Stderr
	return cmd.Run()
}

// textRecord begins intl.sum's record of a message's source text as the registry shipped it: text:screens.name.
const textRecord = "text:"

// follows reports whether the app's text for a message is the registry's own, in an older wording: what the
// registry shipped and recorded, or shipped before it recorded anything. A text the app wrote itself is neither.
func (r *Registry) follows(p *catalog.Project, key, has string) bool {
	return p.Copied(textRecord+key) == catalog.FileHash([]byte(has)) || slices.Contains(r.Superseded[key], has)
}

// addText writes the components' text into the catalogs: the source's, with the comments translators read, and each
// translation the registry ships for a locale the app has. A message the app worded itself keeps its text, and a
// translation goes in only while the app's source says what the registry's does, so it is a translation of it. A
// message the app has as the registry shipped it follows the registry's new wording, in every language. It gives
// the source text of each message that is now the registry's: what intl.sum records, so the next wording is
// followed too.
func (r *Registry) addText(c config.Config, p *catalog.Project, comps []Component) (map[string]string, error) {
	var keys []string
	for _, comp := range comps {
		ks, err := r.Keys(comp)
		if err != nil {
			return nil, err
		}
		for _, k := range ks {
			if !slices.Contains(keys, k) {
				keys = append(keys, k)
			}
		}
	}
	src := c.Locales[0]
	shipped, err := r.Text(src)
	if err != nil {
		return nil, err
	}
	if shipped == nil {
		fmt.Fprintf(os.Stderr, "%s ships its components' text in %s, not %s: [%s] in %s.toml is English until you write it in your language\n",
			r.Command, strings.Join(r.Languages(), ", "), src, r.Table, src)
		if shipped, err = r.Text("en"); err != nil {
			return nil, err
		}
	}
	// The text the app's gsxui components write that its catalogs lack.
	texts, err := p.Gsxui()
	if err != nil {
		return nil, err
	}
	for _, t := range texts {
		if _, ok := shipped[t.Key]; ok && p.Message(t.Key) == nil {
			keys = append(keys, t.Key)
		}
	}
	notes, err := r.Notes()
	if err != nil {
		return nil, err
	}
	var msgs []catalog.NewMessage
	same := map[string]bool{}       // the app's source says what the registry's does
	reworded := map[string]string{} // the registry's text in an older wording: it follows
	for _, k := range keys {
		msgs = append(msgs, catalog.NewMessage{Key: k, Text: shipped[k], Note: notes.Notes[k]})
		m := p.Message(k)
		same[k] = m == nil || m.Text == shipped[k]
		if !same[k] && r.follows(p, k, m.Text) {
			reworded[k], same[k] = shipped[k], true
		}
	}
	report := func(id string, added []string, err error) error {
		if err == nil && len(added) > 0 {
			fmt.Printf("%s.toml: %d messages\n", id, len(added))
		}
		return err
	}
	added, err := catalog.AddMessages(filepath.Join(c.CatalogDir(), src+".toml"), msgs, notes.Tables)
	if err := report(src, added, err); err != nil {
		return nil, err
	}
	if len(reworded) > 0 {
		if err := catalog.SetMessages(filepath.Join(c.CatalogDir(), src+".toml"), reworded); err != nil {
			return nil, err
		}
		fmt.Printf("%s.toml: %d messages reworded, as %s words them now\n", src, len(reworded), r.Command)
	}
	for _, id := range c.Locales[1:] {
		tr, err := r.Text(id)
		if err != nil {
			return nil, err
		}
		var msgs []catalog.NewMessage
		for _, k := range keys {
			if t, ok := tr[k]; ok && same[k] {
				msgs = append(msgs, catalog.NewMessage{Key: k, Text: t})
			}
		}
		added, err := catalog.AddMessages(filepath.Join(c.CatalogDir(), id+".toml"), msgs, nil)
		if err := report(id, added, err); err != nil {
			return nil, err
		}
		again := map[string]string{}
		for k := range reworded {
			if t, ok := tr[k]; ok {
				again[k] = t
			}
		}
		if len(again) > 0 {
			if err := catalog.SetMessages(filepath.Join(c.CatalogDir(), id+".toml"), again); err != nil {
				return nil, err
			}
		}
	}
	records := map[string]string{}
	for _, k := range keys {
		if same[k] {
			records[k] = catalog.FileHash([]byte(shipped[k]))
		}
	}
	return records, nil
}

// unified is a unified diff of a and b, the file at rel, with three lines of context.
func unified(rel string, a, b []byte) string {
	x, y := lines(a), lines(b)
	// lcs[i][j] is the length of the longest common subsequence of x[i:] and y[j:].
	lcs := make([][]int, len(x)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(y)+1)
	}
	for i := len(x) - 1; i >= 0; i-- {
		for j := len(y) - 1; j >= 0; j-- {
			if x[i] == y[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	type op struct {
		kind byte // ' ', '-', '+'
		text string
		i, j int // the line's number in a and in b, from 0
	}
	var ops []op
	i, j := 0, 0
	for i < len(x) || j < len(y) {
		switch {
		case i < len(x) && j < len(y) && x[i] == y[j]:
			ops = append(ops, op{' ', x[i], i, j})
			i, j = i+1, j+1
		case i < len(x) && (j == len(y) || lcs[i+1][j] >= lcs[i][j+1]):
			ops = append(ops, op{'-', x[i], i, j})
			i++
		default:
			ops = append(ops, op{'+', y[j], i, j})
			j++
		}
	}
	var out strings.Builder
	from := "a/" + rel
	if a == nil {
		from = "/dev/null"
	}
	fmt.Fprintf(&out, "--- %s\n+++ b/%s\n", from, rel)
	for k := 0; k < len(ops); {
		if ops[k].kind == ' ' {
			k++
			continue
		}
		start, end := max(k-3, 0), k
		for end < len(ops) {
			if ops[end].kind != ' ' {
				end++
				continue
			}
			run := end
			for run < len(ops) && ops[run].kind == ' ' {
				run++
			}
			if run == len(ops) || run-end > 6 {
				end = min(end+3, len(ops))
				break
			}
			end = run
		}
		na, nb := 0, 0
		for _, o := range ops[start:end] {
			if o.kind != '+' {
				na++
			}
			if o.kind != '-' {
				nb++
			}
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", ops[start].i+1, na, ops[start].j+1, nb)
		for _, o := range ops[start:end] {
			out.WriteString(string(o.kind) + o.text + "\n")
		}
		k = end
	}
	return out.String()
}

func lines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// Behind are the registry's components the app has in dir, has not changed since add wrote them, and that the
// registry now writes differently: what a newer charter improved. Add, for those names, brings them up to date; a
// copy the app changed is its own and is never listed. A copy that is exactly what the registry writes is recorded
// as written by it, whatever the record said: an app made from another (charter create) has every copy under a new
// module path, and they are still charter's.
func (r *Registry) Behind(c config.Config, dir string) ([]string, error) {
	gsxui := cmp.Or(c.Gsxui, setup.DetectGsxui(c.Root))
	if gsxui == "" {
		return nil, nil
	}
	catalogs, err := catalog.PackagePath(c.CatalogDir())
	if err != nil {
		return nil, err
	}
	components, err := catalog.PackagePath(filepath.Join(c.Root, gsxui))
	if err != nil {
		return nil, err
	}
	p, err := catalog.Read(c)
	if err != nil {
		return nil, err
	}
	// Each copy is compared with what add would write now, formatted by the app's own gsx fmt: at once, since each
	// is a program started.
	paths := Paths{Catalogs: catalogs, Gsxui: components}
	n := len(r.Components)
	had, same, errs, rels, olds := make([]bool, n), make([]bool, n), make([]error, n), make([]string, n), make([][]byte, n)
	var wg sync.WaitGroup
	for i, comp := range r.Components {
		rels[i] = filepath.ToSlash(filepath.Join(dir, comp.Name+".gsx"))
		old, err := os.ReadFile(filepath.Join(c.Root, rels[i]))
		if err != nil {
			continue
		}
		had[i], olds[i] = true, old
		wg.Go(func() {
			src, err := r.Source(comp, paths)
			if err == nil {
				src, err = Format(c.Root, rels[i], src)
			}
			same[i], errs[i] = err == nil && bytes.Equal(old, src), err
		})
	}
	wg.Wait()
	var behind []string
	recorded := false
	// A component's text is behind as its file is: the app has a message as the registry shipped it, and the
	// registry words it differently now. What the app has exactly as the registry words it is recorded as such.
	shipped, _ := r.Text(c.Locales[0])
	reworded := func(comp Component) bool {
		keys, err := r.Keys(comp)
		if err != nil || shipped == nil {
			return false
		}
		is := false
		for _, k := range keys {
			m := p.Message(k)
			switch {
			case m == nil:
			case m.Text == shipped[k]:
				if h := catalog.FileHash([]byte(m.Text)); p.Copied(textRecord+k) != h {
					p.SetCopied(textRecord+k, h)
					recorded = true
				}
			case r.follows(p, k, m.Text):
				is = true
			}
		}
		return is
	}
	for i, comp := range r.Components {
		switch {
		case errs[i] != nil:
			return nil, errs[i]
		case !had[i]:
		case same[i]:
			if h := catalog.FileHash(olds[i]); p.Copied(rels[i]) != h {
				p.SetCopied(rels[i], h)
				recorded = true
			}
			if reworded(comp) {
				behind = append(behind, comp.Name)
			}
		case catalog.FileHash(olds[i]) == p.Copied(rels[i]):
			behind = append(behind, comp.Name)
		}
	}
	if recorded {
		if err := p.WriteSum(); err != nil {
			return nil, err
		}
	}
	return behind, nil
}
