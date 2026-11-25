// Package catalog is an app's message catalogs as charter intl sees them (intl.toml's catalogs directory:
// <locale>.toml per shipped locale, ICU MessageFormat, the source locale first): read and checked against the source
// (Read), compiled for package intl (Generate), and what is left to translate (Locale.Todo).
//
// A key missing from a translation comes from the locale intl.toml's [inherit] names for it (en-IN = "en"), so a
// regional catalog holds only what differs; inheriting from the source locale counts as translated. A key with
// nowhere to come from is missing, and shows the source text. intl.sum, beside the catalogs, records the source text
// each translation was made from: when the source changes, the translation is stale until it is updated or
// confirmed. A key the source no longer has is obsolete until charter intl prune removes it.
package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/joeblew999/charter/intl/config"
)

// State is where a locale's text for a message comes from.
type State uint8

const (
	Translated State = iota // the locale's own catalog
	Inherited               // a shipped ancestor's catalog (en-IN ← en)
	Missing                 // nowhere: the source text shows
	Stale                   // a translation of a source text that has changed since
)

func (s State) String() string { return [...]string{"translated", "inherited", "missing", "stale"}[s] }

// Project is an app's catalogs, read and checked.
type Project struct {
	Config   config.Config
	Messages []*Message        // the source's messages in key order: their index in intl-messages.mjs
	Locales  []*Locale         // the shipped locales, the source first
	sum      sum               // intl.sum as it is now
	files    map[string]string // intl.sum's files charter intl add wrote: path → hash
}

// Message is one message of the source catalog.
type Message struct {
	Key     string
	Text    string // ICU MessageFormat
	Context string // the comments above it (and above its table) in the source catalog, for translators
	Args    []Arg  // in first-use order
	parsed  *parsed
	index   map[string]int8
}

// Arg is a message argument: its name and whether it is a "string" or a "number".
type Arg struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// Locale is one shipped locale's text for every message.
type Locale struct {
	ID       string
	File     string   // its catalog
	Entries  []Entry  // one per Project.Messages
	Obsolete []string // keys its catalog has that the source no longer does (charter intl prune)
}

// Entry is a locale's text for one message.
type Entry struct {
	Text   string // the translation, an ancestor's, or the source text when missing
	From   string // the locale Text comes from
	State  State
	parsed *parsed
}

// Count is how many of l's messages are in state s.
func (l *Locale) Count(s State) int {
	n := 0
	for _, e := range l.Entries {
		if e.State == s {
			n++
		}
	}
	return n
}

// Complete reports whether every message is translated (some may be stale): a page in the locale has no source
// text on it.
func (l *Locale) Complete() bool { return l.Count(Missing) == 0 }

// Current reports whether every message is translated from the source as it is now: complete, and nothing stale.
func (l *Locale) Current() bool { return l.Complete() && l.Count(Stale) == 0 }

// Locale is the shipped locale id, compared case-insensitively, or nil.
func (p *Project) Locale(id string) *Locale {
	for _, l := range p.Locales {
		if strings.EqualFold(l.ID, id) {
			return l
		}
	}
	return nil
}

// Message is the source message key, or nil.
func (p *Project) Message(key string) *Message {
	if i := slices.IndexFunc(p.Messages, func(m *Message) bool { return m.Key == key }); i >= 0 {
		return p.Messages[i]
	}
	return nil
}

// Read reads and checks c's catalogs: every message parses, and a translation uses the source's arguments with the
// same kinds and no key the source lacks.
func Read(c config.Config) (*Project, error) {
	if len(c.Locales) == 0 || c.Catalogs == "" {
		return nil, errors.New("intl: intl.toml needs locales and catalogs")
	}
	dir, source := c.CatalogDir(), c.Locales[0]
	srcFile := filepath.Join(dir, source+".toml")
	src, err := readCatalog(srcFile)
	if err != nil {
		return nil, err
	}
	notes, err := contexts(srcFile)
	if err != nil {
		return nil, err
	}
	p := &Project{Config: c, sum: sum{}}
	for _, key := range sortedKeys(src) {
		pm, err := parse(src[key])
		if err != nil {
			return nil, fmt.Errorf("intl: %s.toml %s: %w", source, key, err)
		}
		m := &Message{Key: key, Text: src[key], Context: notes[key], parsed: pm, index: map[string]int8{}}
		for i, name := range pm.order {
			m.index[name] = int8(i)
			m.Args = append(m.Args, Arg{Name: name, Kind: kindName(pm.args[name])})
		}
		p.Messages = append(p.Messages, m)
	}
	own := map[string]map[string]string{source: src}
	obsolete := map[string][]string{}
	for _, id := range c.Locales[1:] {
		tr, err := readCatalog(filepath.Join(dir, id+".toml"))
		if errors.Is(err, os.ErrNotExist) {
			tr = map[string]string{}
		} else if err != nil {
			return nil, err
		}
		for _, k := range sortedKeys(tr) {
			if p.Message(k) == nil {
				obsolete[id] = append(obsolete[id], k)
				delete(tr, k)
			}
		}
		own[id] = tr
	}

	// Staleness: a translation is of the source text intl.sum recorded with it. A changed translation is taken to
	// be of the current source.
	old, files, err := readSum(p.SumFile())
	if err != nil {
		return nil, err
	}
	p.files = files
	stale := map[string]bool{} // "id key"
	for _, id := range c.Locales[1:] {
		for _, m := range p.Messages {
			tr, ok := own[id][m.Key]
			if !ok {
				continue
			}
			e, had := old.get(id, m.Key)
			switch {
			case !had || e.translation != hash(tr):
				e = sumEntry{source: hash(m.Text), translation: hash(tr)}
			case e.source != hash(m.Text):
				stale[id+" "+m.Key] = true
			}
			p.sum.set(id, m.Key, e)
		}
	}

	for _, id := range c.Locales {
		l := &Locale{ID: id, File: filepath.Join(dir, id+".toml"), Obsolete: obsolete[id]}
		for _, m := range p.Messages {
			e := Entry{Text: m.Text, From: source, State: Missing, parsed: m.parsed}
			for from, seen := id, map[string]bool{}; from != "" && !seen[from]; from = parent(c, from) {
				seen[from] = true
				text, ok := own[from][m.Key]
				if !ok {
					continue
				}
				e.Text, e.From, e.State = text, from, Translated
				if from != id {
					e.State = Inherited
				}
				if stale[from+" "+m.Key] {
					e.State = Stale
				}
				break
			}
			if e.From != source {
				if e.parsed, err = parse(e.Text); err == nil {
					err = m.checkArgs(e.parsed)
				}
				if err != nil {
					return nil, fmt.Errorf("intl: %s.toml %s: %w", e.From, m.Key, err)
				}
			}
			l.Entries = append(l.Entries, e)
		}
		p.Locales = append(p.Locales, l)
	}
	return p, nil
}

// Todo is what is left to translate in l: its missing messages and its own stale translations (an inherited stale
// one is its ancestor's to fix).
func (l *Locale) Todo(p *Project) []Todo {
	var out []Todo
	for i, e := range l.Entries {
		if e.State == Missing || e.State == Stale && e.From == l.ID {
			t := Todo{Message: p.Messages[i], State: e.State}
			if e.State == Stale {
				t.Translation = e.Text
			}
			out = append(out, t)
		}
	}
	return out
}

// Todo is a message to translate, with the stale translation to update or confirm.
type Todo struct {
	*Message
	State       State
	Translation string
}

// Confirm keeps l's stale translations of keys (every stale one when keys is empty) as translations of the
// current source, and writes intl.sum.
func (p *Project) Confirm(l *Locale, keys ...string) (int, error) {
	n := 0
	for i, e := range l.Entries {
		m := p.Messages[i]
		if e.State != Stale || e.From != l.ID || len(keys) > 0 && !slices.Contains(keys, m.Key) {
			continue
		}
		s, _ := p.sum.get(l.ID, m.Key)
		s.source = hash(m.Text)
		p.sum.set(l.ID, m.Key, s)
		l.Entries[i].State = Translated
		n++
	}
	for _, k := range keys {
		if p.Message(k) == nil {
			return n, fmt.Errorf("intl: no message %s", k)
		}
	}
	return n, p.WriteSum()
}

// Plural is a plural or selectordinal in a message, with the categories it has a case for.
type Plural struct {
	Ordinal bool
	Cases   []string
}

// Plurals are e's plurals and selectordinals, nested ones included.
func (e Entry) Plurals() []Plural {
	var out []Plural
	var walk func([]node)
	walk = func(nodes []node) {
		for _, n := range nodes {
			if n.kind == kPlural || n.kind == kSelectOrdinal {
				pl := Plural{Ordinal: n.kind == kSelectOrdinal}
				for _, c := range n.cases {
					pl.Cases = append(pl.Cases, c.key)
				}
				out = append(out, pl)
			}
			for _, c := range n.cases {
				walk(c.parts)
			}
		}
	}
	walk(e.parsed.parts)
	return out
}

func readCatalog(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if _, err := toml.Decode(string(b), &raw); err != nil {
		return nil, fmt.Errorf("intl: %s: %w", path, err)
	}
	out := map[string]string{}
	var walk func(prefix string, m map[string]any) error
	walk = func(prefix string, m map[string]any) error {
		for k, v := range m {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			switch x := v.(type) {
			case string:
				out[key] = x
			case map[string]any:
				if err := walk(key, x); err != nil {
					return err
				}
			default:
				return fmt.Errorf("intl: %s: %s is not a string", path, key)
			}
		}
		return nil
	}
	return out, walk("", raw)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func kindName(k argKind) string {
	if k == argNumber {
		return "number"
	}
	return "string"
}

// parent is the shipped locale id takes what it doesn't translate from (intl.toml's [inherit]), or "".
func parent(c config.Config, id string) string {
	if p := c.Inherit[id]; slices.Contains(c.Locales, p) {
		return p
	}
	return ""
}
