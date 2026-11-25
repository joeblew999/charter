package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joeblew999/charter/intl/config"
)

// SetMessages writes msgs (key → ICU MessageFormat) into the catalog at path, created if need be. A key already there
// gets its new text in place; a new one goes at the end of its table, which is added if need be. Comments, order
// and everything else stay as they are.
func SetMessages(path string, msgs map[string]string) error {
	lines, err := readLines(path)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(msgs))
	for k := range msgs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, key := range keys {
		table, leaf := splitKey(key)
		layout := scan(lines)
		if at, ok := layout.keys[key]; ok {
			// Keep the key as the file writes it (notes.title in [board]): only its value changes.
			name, _, _ := strings.Cut(lines[at.start], "=")
			lines = slices.Replace(lines, at.start, at.end+1, strings.TrimRight(name, " \t")+" = "+tomlString(msgs[key]))
			continue
		}
		lines = insert(lines, layout, table, []string{tomlKey(leaf) + " = " + tomlString(msgs[key])}, nil)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// NewMessage is a message for AddMessages: its key, its text (ICU MessageFormat) and the comment lines above it.
type NewMessage struct {
	Key, Text string
	Note      []string
}

// AddMessages writes each of msgs that the catalog at path (created if need be) doesn't have yet, in order, at the end
// of its table, with its note above it; a table it adds goes at the end, tables[name] above its header. A key the
// catalog has keeps its text: the app may have changed it. It returns the keys it wrote.
func AddMessages(path string, msgs []NewMessage, tables map[string][]string) ([]string, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	var added []string
	for _, m := range msgs {
		layout := scan(lines)
		if _, ok := layout.keys[m.Key]; ok {
			continue
		}
		table, leaf := splitKey(m.Key)
		block := append(comments(m.Note), tomlKey(leaf)+" = "+tomlString(m.Text))
		lines = insert(lines, layout, table, block, comments(tables[table]))
		added = append(added, m.Key)
	}
	if len(added) == 0 {
		return nil, nil
	}
	return added, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func readLines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s := strings.TrimRight(string(b), "\n"); s != "" {
		return strings.Split(s, "\n"), nil
	}
	return nil, nil
}

// splitKey is a key's table and its name in it: board.notes is notes in [board].
func splitKey(key string) (table, leaf string) {
	if i := strings.LastIndexByte(key, '.'); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

// insert puts block after the last key of table, or right under its header when it has none. A table the catalog
// doesn't have is added at the end, below note.
func insert(lines []string, l layout, table string, block, note []string) []string {
	sec, ok := l.tables[table]
	if !ok {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, note...)
		if table != "" {
			lines = append(lines, "["+table+"]")
		}
		return append(lines, block...)
	}
	at := sec.start
	for _, k := range l.keys {
		if k.start >= sec.start && k.end < sec.end && k.end+1 > at {
			at = k.end + 1
		}
	}
	return slices.Insert(lines, at, block...)
}

// comments are note's lines as TOML comments.
func comments(note []string) []string {
	out := make([]string, len(note))
	for i, n := range note {
		out[i] = strings.TrimRight("# "+n, " ")
	}
	return out
}

type span struct{ start, end int } // lines; a table's end is exclusive, a key's inclusive

type layout struct {
	tables map[string]span // "" is the top level, before the first table
	keys   map[string]span
}

func scan(lines []string) layout {
	l := layout{tables: map[string]span{}, keys: map[string]span{}}
	table, start := "", 0
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		switch {
		case t == "" || t[0] == '#':
		case t[0] == '[':
			l.tables[table] = span{start, i}
			table, start = tableName(t), i+1
		default:
			name, value, ok := strings.Cut(t, "=")
			if !ok {
				continue
			}
			key := strings.Trim(strings.TrimSpace(name), `"`)
			if table != "" {
				key = table + "." + key
			}
			end := i
			v := strings.TrimSpace(value)
			for _, q := range []string{`"""`, `'''`} {
				if strings.HasPrefix(v, q) && !strings.Contains(v[3:], q) {
					for end+1 < len(lines) && !strings.Contains(lines[end+1], q) {
						end++
					}
					end++
				}
			}
			l.keys[key] = span{i, min(end, len(lines)-1)}
			i = end
		}
	}
	l.tables[table] = span{start, len(lines)}
	return l
}

// tableName is a table header's name: "[board] # the board" is board.
func tableName(header string) string {
	if i := strings.IndexByte(header, ']'); i >= 0 {
		header = header[:i]
	}
	return strings.Trim(strings.TrimSpace(strings.TrimPrefix(header, "[")), `"`)
}

// RemoveMessages deletes keys from the catalog at path (charter intl prune: keys the source no longer has), each with
// the comment above it, and a table they leave without keys, with the comment above its header.
func RemoveMessages(path string, keys []string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	l := scan(lines)
	drop := make([]bool, len(lines))
	mark := func(from, to int) {
		for i := from; i < to; i++ {
			drop[i] = true
		}
	}
	for _, k := range keys {
		if sp, ok := l.keys[k]; ok {
			mark(noted(lines, sp.start), sp.end+1)
		}
	}
	for name, sec := range l.tables {
		removed, kept := 0, 0
		for k, sp := range l.keys {
			if sp.start >= sec.start && sp.end < sec.end {
				if slices.Contains(keys, k) {
					removed++
				} else {
					kept++
				}
			}
		}
		if name == "" || removed == 0 || kept > 0 {
			continue
		}
		// From the comment above its header to the comment above the next one.
		end := sec.end
		for end > sec.start && strings.HasPrefix(strings.TrimSpace(lines[end-1]), "#") {
			end--
		}
		mark(noted(lines, sec.start-1), end)
	}
	var out []string
	for i, line := range lines {
		if !drop[i] {
			out = append(out, line)
		}
	}
	return os.WriteFile(path, []byte(strings.TrimRight(strings.Join(out, "\n"), "\n")+"\n"), 0o644)
}

// noted is line i, or the first line of the comment directly above it.
func noted(lines []string, i int) int {
	for i > 0 && strings.HasPrefix(strings.TrimSpace(lines[i-1]), "#") {
		i--
	}
	return i
}

var bareKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func tomlKey(k string) string {
	if bareKey.MatchString(k) {
		return k
	}
	return tomlString(k)
}

// tomlString is s as a TOML basic string.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

var localesLine = regexp.MustCompile(`(?ms)^(locales\s*=\s*\[)(.*?)(\])`)

// AddLocale adds id to intl.toml's locales and creates its catalog, headed with the language's name.
func AddLocale(c config.Config, id, name string) error {
	if slices.ContainsFunc(c.Locales, func(l string) bool { return strings.EqualFold(l, id) }) {
		return fmt.Errorf("intl: %s is already in %s", id, config.File)
	}
	path := filepath.Join(c.Root, config.File)
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m := localesLine.FindSubmatchIndex(b)
	if m == nil {
		return fmt.Errorf("intl: %s: no locales = [...] to add %s to", path, id)
	}
	list := strings.TrimRight(string(b[m[4]:m[5]]), " \t\n,")
	out := string(b[:m[4]]) + list + `, "` + id + `"` + string(b[m[5]:])
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	file := filepath.Join(c.CatalogDir(), id+".toml")
	if _, err := os.Stat(file); err == nil {
		return nil
	}
	head := fmt.Sprintf("# %s (%s): the translations of %s.toml, same tables and keys. charter intl todo lists what is left.\n",
		name, id, c.Locales[0])
	return os.WriteFile(file, []byte(head), 0o644)
}
