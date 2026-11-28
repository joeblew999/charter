package screens

import (
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/joeblew999/charter/internal/docsgen"
	"github.com/joeblew999/charter/intl/registry"
)

// TestSource: a screen's imports point at the app's packages and come out sorted, the catalogs' package aliased to
// locales when it has another name.
func TestSource(t *testing.T) {
	for _, tc := range []struct {
		paths registry.Paths
		want  string
	}{
		{registry.Paths{Catalogs: "example.com/app/locales", Gsxui: "example.com/app/ui"},
			"import (\n\t\"example.com/app/locales\"\n\t\"example.com/app/ui/icon\"\n\tui \"example.com/app/ui/styles\"\n\t\"github.com/gsxhq/gsx\"\n\t\"github.com/joeblew999/charter/intl\"\n)\n"},
		{registry.Paths{Catalogs: "zzz.example/app/messages", Gsxui: "zzz.example/app/components"},
			"import (\n\t\"github.com/gsxhq/gsx\"\n\t\"github.com/joeblew999/charter/intl\"\n\t\"zzz.example/app/components/icon\"\n\tui \"zzz.example/app/components/styles\"\n\tlocales \"zzz.example/app/messages\"\n)\n"},
	} {
		s, _ := Screens.Find("language-list")
		b, err := Screens.Source(s, tc.paths)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), tc.want) {
			t.Errorf("%v: imports\n%s\nwant\n%s", tc.paths, b[:strings.Index(string(b), ")\n")+2], tc.want)
		}
		if strings.Contains(string(b), "intl/ui/locales") || strings.Contains(string(b), "gsxhq/gsxui") {
			t.Errorf("%v: a template import is left", tc.paths)
		}
	}
}

// TestKeys: every message a screen writes is in the source text, and every language intl ships translates all of
// them, and the text of gsxui's own components.
func TestKeys(t *testing.T) {
	src, err := Screens.Text("en")
	if err != nil {
		t.Fatal(err)
	}
	all := map[string]bool{}
	for _, s := range Screens.Components {
		keys, err := Screens.Keys(s)
		if err != nil {
			t.Fatal(err)
		}
		if len(keys) == 0 {
			t.Errorf("%s writes no messages", s.Name)
		}
		for _, k := range keys {
			all[k] = true
		}
	}
	for k := range src {
		if strings.HasPrefix(k, "screens.") && !all[k] {
			t.Errorf("no screen writes %s", k)
		}
	}
	for _, id := range Screens.Languages()[1:] {
		tr, err := Screens.Text(id)
		if err != nil {
			t.Fatal(err)
		}
		for k := range src {
			if _, ok := tr[k]; !ok {
				t.Errorf("%s has no %s", id, k)
			}
		}
		for k := range tr {
			if _, ok := src[k]; !ok {
				t.Errorf("%s has %s, which en doesn't", id, k)
			}
		}
	}
	if !slices.Contains(Screens.Languages(), "ar") || Screens.Languages()[0] != "en" {
		t.Errorf("Languages() = %v", Screens.Languages())
	}
}

// The classes charter build writes into a docs page's HTML (a list's, a link's: gsxui's own doc pages', in
// internal/docsgen) are in the docs screen's text too, which is where Tailwind reads them from.
func TestTheDocsScreenHasTheClassesOfAPage(t *testing.T) {
	b, err := fs.ReadFile(Screens.Files, "docs.gsx.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	_, declared, ok := strings.Cut(string(b), "const docsClasses = ")
	if !ok {
		t.Fatal("the docs screen no longer says the classes of a page (docsClasses)")
	}
	declared, _, _ = strings.Cut(declared, "\n")
	has := strings.Fields(strings.Trim(declared, `"`))
	for _, classes := range docsgen.Classes {
		for _, class := range strings.Fields(classes) {
			if !slices.Contains(has, class) {
				t.Errorf("charter build gives a docs page the class %s, which the docs screen does not have for Tailwind", class)
			}
		}
	}
}
