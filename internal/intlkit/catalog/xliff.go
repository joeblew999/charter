package catalog

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

// XLIFF 2.0 (OASIS), the format translators' tools and translation-management systems read and write: one file per
// locale, a unit per message, its id the message key. Source and target are ICU MessageFormat as written in the
// catalogs; notes carry the context, the arguments and what the language's plurals need.
type xliffDoc struct {
	XMLName xml.Name  `xml:"urn:oasis:names:tc:xliff:document:2.0 xliff"`
	Version string    `xml:"version,attr"`
	SrcLang string    `xml:"srcLang,attr"`
	TrgLang string    `xml:"trgLang,attr"`
	File    xliffFile `xml:"file"`
}

type xliffFile struct {
	ID       string      `xml:"id,attr"`
	Original string      `xml:"original,attr,omitempty"`
	Units    []xliffUnit `xml:"unit"`
}

type xliffUnit struct {
	ID      string       `xml:"id,attr"`
	Notes   []xliffNote  `xml:"notes>note,omitempty"`
	Segment xliffSegment `xml:"segment"`
}

type xliffNote struct {
	Category string `xml:"category,attr,omitempty"`
	Text     string `xml:",chardata"`
}

type xliffSegment struct {
	State  string  `xml:"state,attr,omitempty"`
	Source string  `xml:"source"`
	Target *string `xml:"target"`
}

// ExportXLIFF is l's messages as XLIFF 2.0: what is left to translate, or every message with all. plurals, if
// set, is a note on what the language's plural and selectordinal cases must be.
func (p *Project) ExportXLIFF(l *Locale, all bool, plurals string) ([]byte, int, error) {
	if l == p.Locales[0] {
		return nil, 0, errors.New("intl: the source locale has nothing to translate")
	}
	doc := xliffDoc{Version: "2.0", SrcLang: p.Locales[0].ID, TrgLang: l.ID,
		File: xliffFile{ID: p.Config.Catalogs, Original: p.Config.Catalogs + "/" + l.ID + ".toml"}}
	for i, e := range l.Entries {
		m := p.Messages[i]
		todo := e.State == Missing || e.State == Stale && e.From == l.ID
		if !todo && (!all || e.From != l.ID) {
			continue
		}
		u := xliffUnit{ID: m.Key, Segment: xliffSegment{Source: m.Text, State: "translated"}}
		if m.Context != "" {
			u.Notes = append(u.Notes, xliffNote{Category: "context", Text: m.Context})
		}
		if len(m.Args) > 0 {
			var args []string
			for _, a := range m.Args {
				args = append(args, "{"+a.Name+"} "+a.Kind)
			}
			u.Notes = append(u.Notes, xliffNote{Category: "arguments", Text: "Keep these as they are: " + strings.Join(args, ", ")})
		}
		if plurals != "" && len(e.Plurals()) > 0 {
			u.Notes = append(u.Notes, xliffNote{Category: "plurals", Text: plurals})
		}
		switch e.State {
		case Missing:
			u.Segment.State = "initial"
		case Stale:
			u.Segment.State = "initial"
			u.Notes = append(u.Notes, xliffNote{Category: "stale", Text: "The source changed since this was translated: update the target, or mark it translated as it is."})
		}
		if e.State != Missing {
			t := e.Text
			u.Segment.Target = &t
		}
		doc.File.Units = append(doc.File.Units, u)
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	return append([]byte(xml.Header), append(out, '\n')...), len(doc.File.Units), nil
}

// ImportXLIFF writes the translated targets of an XLIFF 2.0 file (state translated, reviewed or final, or none) into
// its locale's catalog, after checking each against the source. A target that is the stale translation unchanged
// is kept as it is (Confirm). It reports the locale and how many messages changed and were confirmed; the caller
// regenerates.
func (p *Project) ImportXLIFF(data []byte) (l *Locale, changed, confirmed int, err error) {
	var doc xliffDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, 0, 0, fmt.Errorf("intl: not XLIFF 2.0: %w", err)
	}
	if l = p.Locale(doc.TrgLang); l == nil || l == p.Locales[0] {
		return nil, 0, 0, fmt.Errorf("intl: trgLang %q is not a shipped translation locale", doc.TrgLang)
	}
	set := map[string]string{}
	var keep []string
	var problems []string
	for _, u := range doc.File.Units {
		if u.Segment.Target == nil || strings.TrimSpace(*u.Segment.Target) == "" {
			continue
		}
		switch u.Segment.State {
		case "", "translated", "reviewed", "final":
		default:
			continue
		}
		m := p.Message(u.ID)
		if m == nil {
			problems = append(problems, u.ID+": no such message")
			continue
		}
		text := *u.Segment.Target
		if u.Segment.Source != m.Text {
			// Translated from older source text: taking it as current would hide that it is stale.
			problems = append(problems, u.ID+": translated from an older source text (export again)")
			continue
		}
		if err := m.check(text); err != nil {
			problems = append(problems, u.ID+": "+err.Error())
			continue
		}
		e := l.Entries[p.index(m)]
		switch {
		case e.From == l.ID && e.Text == text && e.State == Stale:
			keep = append(keep, m.Key)
		case e.From != l.ID || e.Text != text:
			set[m.Key] = text
		}
	}
	if len(problems) > 0 {
		return l, 0, 0, errors.New("intl: " + l.ID + ": " + strings.Join(problems, "; "))
	}
	if len(set) > 0 {
		if err := SetMessages(l.File, set); err != nil {
			return l, 0, 0, err
		}
	}
	if len(keep) > 0 {
		if confirmed, err = p.Confirm(l, keep...); err != nil {
			return l, len(set), confirmed, err
		}
	}
	return l, len(set), confirmed, nil
}

func (p *Project) index(m *Message) int {
	for i, x := range p.Messages {
		if x == m {
			return i
		}
	}
	return -1
}

// check reports whether text is a valid translation of m: it parses, and uses only m's arguments, as m does.
func (m *Message) check(text string) error {
	t, err := parse(text)
	if err != nil {
		return err
	}
	return m.checkArgs(t)
}

func (m *Message) checkArgs(t *parsed) error {
	for name, kind := range t.args {
		want, known := m.parsed.args[name]
		if !known {
			return fmt.Errorf("argument {%s} isn't in the source", name)
		}
		if kind != want {
			return fmt.Errorf("argument {%s} is used as a %s, the source uses it as a %s", name, kindName(kind), kindName(want))
		}
	}
	return nil
}
