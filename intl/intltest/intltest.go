// Package intltest gives go test the Intl a Worker has: intl's js/intl-worker.mjs, with the app's catalogs, on
// workerd (intl/workerd), set as intl.Runtime. There is no other Intl: a package whose tests render or format anything
// runs them under it from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(intltest.Main(m)) }
//
// It is the repo's intl daemon (workerd.Start: pitchfork runs it, with the catalogs mise-intl gen wrote).
package intltest

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/intl/config"
	"github.com/joeblew999/charter/intl/workerd"
)

// Main runs the tests with workerd's Intl as intl.Runtime: the repo's intl daemon, with the catalogs gen wrote
// (workerd.Start).
func Main(m *testing.M) int {
	if err := workerd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "intltest:", err)
		return 1
	}
	return m.Run()
}

// Plurals checks that every plural and selectordinal in the app's catalogs has a case for each category its
// language's Intl.PluralRules uses (Russian needs one, few, many and other):
//
//	func TestPlurals(t *testing.T) { intltest.Plurals(t) }
func Plurals(t testing.TB) {
	t.Helper()
	c, err := config.Find(".")
	if err != nil {
		t.Fatal(err)
	}
	p, err := catalog.Read(c)
	if err != nil {
		t.Fatal(err)
	}
	problems, err := PluralProblems(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range problems {
		t.Error(s)
	}
}

// Categories are the plural categories Intl.PluralRules uses for locale id: cardinal ("one", "few", "many",
// "other" for Russian) and ordinal.
func Categories(id string) (cardinal, ordinal []string, err error) {
	get := func(typ string) ([]string, error) {
		s, ok := intl.NewLocale(id).Intl(intl.Call{API: "PluralRules", Method: "resolvedOptions", Opts: intl.Options("type", typ)}, "", "")
		var o struct{ PluralCategories []string }
		if !ok || json.Unmarshal([]byte(s), &o) != nil || len(o.PluralCategories) == 0 {
			return nil, fmt.Errorf("intl: Intl has no %s plural categories for %s", typ, id)
		}
		return o.PluralCategories, nil
	}
	if cardinal, err = get("cardinal"); err != nil {
		return nil, nil, err
	}
	ordinal, err = get("ordinal")
	return cardinal, ordinal, err
}

// PluralProblems are the plurals and selectordinals in p's own translations (and the source) missing a case for a
// category their language uses. Intl must be running (Start, StartIntl).
func PluralProblems(p *catalog.Project) ([]string, error) {
	var out []string
	for _, l := range p.Locales {
		cardinal, ordinal, err := Categories(l.ID)
		if err != nil {
			return nil, err
		}
		for i, e := range l.Entries {
			if e.From != l.ID {
				continue // missing, or an ancestor's: checked there
			}
			for _, pl := range e.Plurals() {
				need, typ := cardinal, "plural"
				if pl.Ordinal {
					need, typ = ordinal, "selectordinal"
				}
				for _, cat := range need {
					if !slices.Contains(pl.Cases, cat) {
						out = append(out, fmt.Sprintf("%s %s: a %s without a %q case (%s needs %s)",
							l.File, p.Messages[i].Key, typ, cat, l.ID, strings.Join(need, ", ")))
					}
				}
			}
		}
	}
	return out, nil
}

// Tree formats a message tree as mise-intl gen writes it (js/intl.mjs message) in lang with args.
func Tree(lang string, tree json.RawMessage, args ...string) (string, error) {
	return workerd.Tree(lang, tree, args...)
}
