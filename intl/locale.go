package intl

import (
	"strings"
	"sync"
	"time"
)

// Locale is a shipped locale: its BCP 47 id, and what Intl says about it (its name in its own language, text
// direction, likely subtags), asked once per runtime and kept: it is the same for every request. Make one per id
// with NewLocale and share it; the generated package's Shipped holds the app's.
type Locale struct {
	id                 string
	once               sync.Once
	name, dir, maximal string

	mu    sync.Mutex
	plain map[messageKey]string // messages without arguments, as Intl formatted them

	calOnce   sync.Once
	cal       Calendar
	weekStart time.Weekday
}

// NewLocale returns the locale with a BCP 47 id, e.g. "pt-BR".
func NewLocale(id string) *Locale { return &Locale{id: id} }

// ID is the locale id, e.g. "pt-BR": for <html lang>, hreflang and Intl.
func (l *Locale) ID() string { return l.id }

// Name is the locale's name in its own language (Intl.DisplayNames), e.g. "português (Brasil)".
func (l *Locale) Name() string { l.ask(); return l.name }

// Dir is "rtl" or "ltr" (Intl.Locale's text direction), for <html dir>.
func (l *Locale) Dir() string { l.ask(); return l.dir }

// Maximal is the id with its likely subtags (Intl.Locale maximize), e.g. "pt-Latn-BR".
func (l *Locale) Maximal() string { l.ask(); return l.maximal }

func (l *Locale) ask() {
	l.once.Do(func() {
		get := func(c Call, none string) string {
			if s, ok := l.Intl(c, "", ""); ok {
				return s
			}
			return none
		}
		l.name = get(Call{API: "DisplayNames", Method: "of", Opts: Options("type", "language"), Args: []string{JSON(l.id)}}, l.id)
		l.dir = "ltr"
		if strings.Contains(get(Call{API: "Locale", Method: "getTextInfo"}, ""), `"rtl"`) {
			l.dir = "rtl"
		}
		l.maximal = get(Call{API: "Locale", Method: "maximize"}, l.id)
	})
}

// Locales are the locales an app ships, its source (default) locale first.
type Locales []*Locale

// Find returns the shipped locale with id, compared case-insensitively ("pt-br" finds pt-BR), or nil.
func (s Locales) Find(id string) *Locale {
	for _, l := range s {
		if strings.EqualFold(l.id, id) {
			return l
		}
	}
	return nil
}

// The app's catalogs, which its generated package registers (Use).
var (
	shipped  Locales
	complete map[string]bool
)

// Use registers the app's locales, the source first, each with its catalog and whether that translates every
// message. The package mise-intl gen writes calls it from init.
func Use(locales Locales, completeCatalogs []bool) {
	shipped, complete = locales, map[string]bool{}
	for i, l := range locales {
		complete[l.id] = completeCatalogs[i]
	}
}

// Complete reports whether the catalog of the locale id translates every message (a partial one is better served
// noindex).
func Complete(id string) bool { return complete[id] }

// Default is the source locale: the language of a render without a request.
func Default() *Locale {
	if len(shipped) == 0 {
		panic("intl: no locales: import the package mise-intl gen writes")
	}
	return shipped[0]
}
