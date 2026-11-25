package catalog_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
	"github.com/joeblew999/charter/site/intl/config"
	"github.com/joeblew999/charter/site/intl/intltest"
)

func TestMain(m *testing.M) { os.Exit(intltest.Main(m)) }

// TestParseAndFormat: a parsed message, as gen writes it, formatted by js/intl.mjs with workerd's Intl.
func TestParseAndFormat(t *testing.T) {
	type args = map[string]string
	for _, tc := range []struct {
		locale, src string
		args        args
		want        string
	}{
		{"en", "It''s {name}", args{"name": "Ada"}, "It's \u2068Ada\u2069"},
		// Another message (Translate marks it U+E001) is in the page's own language: no isolates, as in a <title>.
		{"en", "{title} · go-htmx4", args{"title": "\ue001Home"}, "Home · go-htmx4"},
		{"en", "'{'literal'}' # {n, number}", args{"n": "1234.5"}, "{literal} # 1,234.5"},
		{"en", "'{n}' is quoted, don't", nil, "{n} is quoted, don't"},
		{"de", "{n, number}", args{"n": "1234.5"}, "1.234,5"},
		{"en", "{n, number, integer}", args{"n": "2.5"}, "3"},
		{"en-IN", "{n, number}", args{"n": "1234567"}, "12,34,567"},
		{"es", "{n, number}", args{"n": "1234"}, "1234"},
		{"es", "{n, number}", args{"n": "12345"}, "12.345"},
		{"en", "{n, number}", args{"n": "0.12345"}, "0.123"},
		{"en", "{n, plural, =0 {none} one {# item} other {# items}}", args{"n": "0"}, "none"},
		{"en", "{n, plural, =0 {none} one {# item} other {# items}}", args{"n": "1"}, "1 item"},
		{"en", "{n, plural, =0 {none} one {# item} other {# items}}", args{"n": "1234"}, "1,234 items"},
		{"ru", "{n, plural, one {# символ} few {# символа} many {# символов} other {# символа}}", args{"n": "3"}, "3 символа"},
		{"ru", "{n, plural, one {# символ} few {# символа} many {# символов} other {# символа}}", args{"n": "5"}, "5 символов"},
		{"ru", "{n, plural, one {# символ} few {# символа} many {# символов} other {# символа}}", args{"n": "1.5"}, "1,5 символа"},
		{"en", "{n, plural, offset:1 =0 {nobody} =1 {{who}} one {{who} and # other} other {{who} and # others}}", args{"n": "1", "who": "Ada"}, "\u2068Ada\u2069"},
		{"en", "{n, plural, offset:1 =0 {nobody} =1 {{who}} one {{who} and # other} other {{who} and # others}}", args{"n": "2", "who": "Ada"}, "\u2068Ada\u2069 and 1 other"},
		{"en", "{n, plural, offset:1 =0 {nobody} =1 {{who}} one {{who} and # other} other {{who} and # others}}", args{"n": "3", "who": "Ada"}, "\u2068Ada\u2069 and 2 others"},
		{"en", "{n, selectordinal, one {#st} two {#nd} few {#rd} other {#th}}", args{"n": "22"}, "22nd"},
		{"en", "{n, selectordinal, one {#st} two {#nd} few {#rd} other {#th}}", args{"n": "13"}, "13th"},
		{"en", "{g, select, female {her} male {his} other {their}} turn", args{"g": "x"}, "their turn"},
		{"en", "{g, select, female {her} male {his} other {their}} turn", args{"g": "female"}, "her turn"},
		// select matches the value itself, in any direction; a text argument is isolated where it lands.
		{"ar", "{role, select, admin {مدير} other {عضو}}", args{"role": "admin"}, "مدير"},
		{"en", "{on, select, 1 {yes} other {no}}", args{"on": "1"}, "yes"},
		{"ar", "مرحبا {name}", args{"name": "Ada"}, "مرحبا \u2068Ada\u2069"},
	} {
		tree, order, err := catalog.Tree(tc.src)
		if err != nil {
			t.Errorf("parse(%q): %v", tc.src, err)
			continue
		}
		vals := make([]string, len(order))
		for i, name := range order {
			vals[i] = tc.args[name]
		}
		got, err := intltest.Tree(tc.locale, tree, vals...)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%s %q = %q, want %q", tc.locale, tc.src, got, tc.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{"{n", "a } b", "{}", "{n, plural, one {# file}}", "{n, when}", "{n, date}", "{n, number, percent}", "{n, number, ::currency/EUR}"} {
		if catalog.ParseError(src) == nil {
			t.Errorf("parse(%q) succeeded, want an error", src)
		}
	}
}

// app is a throwaway app: go.mod, and files written relative to its root.
type app struct {
	t    *testing.T
	root string
}

func newApp(t *testing.T) app {
	a := app{t, t.TempDir()}
	a.write("go.mod", "module example.com/app\n")
	return a
}

func (a app) write(name, s string) {
	path := filepath.Join(a.root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		a.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		a.t.Fatal(err)
	}
}

func (a app) read(name string) string {
	b, err := os.ReadFile(filepath.Join(a.root, name))
	if err != nil {
		a.t.Fatal(err)
	}
	return string(b)
}

func (a app) project(locales ...string) *catalog.Project {
	a.t.Helper()
	p, err := catalog.Read(config.Config{Locales: locales, Catalogs: "cat", Gsxui: "ui", Root: a.root, Inherit: map[string]string{"en-IN": "en"}})
	if err != nil {
		a.t.Fatal(err)
	}
	return p
}

func TestGenerate(t *testing.T) {
	a := newApp(t)
	a.write("cat/en.toml", "hello = \"Hello, {name}\"\ntitle = \"Files\"\n[files]\ncount = \"{n, plural, one {# file} other {# files}}\"\n")
	a.write("cat/en-IN.toml", "# only differences\n")
	a.write("cat/de.toml", "hello = \"Hallo, {name}\"\n")
	var log strings.Builder
	if err := a.project("en", "en-IN", "de").Generate(&log); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"en       3/3 translated\n", "en-IN    3/3 translated, 3 inherited\n", "de       1/3 translated, 2 missing\n"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log.String())
		}
	}
	goSrc := a.read("cat/intl_gen.go")
	for _, want := range []string{"package cat", `var Shipped = intl.Locales{intl.NewLocale("en"), intl.NewLocale("en-IN"), intl.NewLocale("de")}`,
		"func init() { intl.Use(Shipped, []bool{true, true, false}) }",
		"func Hello[T0 intl.TextArg](name T0) intl.Msg { return intl.M(1, intl.Str(name)) }",
		"func FilesCount[T0 intl.NumberArg](n T0) intl.Msg { return intl.M(0, intl.Num(n)) }",
		"var Title = intl.M(2)"} {
		if !strings.Contains(goSrc, want) {
			t.Errorf("intl_gen.go lacks %q:\n%s", want, goSrc)
		}
	}
	js := a.read("cat/intl-messages.mjs")
	for _, want := range []string{`"de": [`, `["p",0,0,{"one":[["#"]," file"],"other":[["#"]," files"]}]`, `"Hallo, ",["a",0]`, `"en-XA": [`} {
		if !strings.Contains(js, want) {
			t.Errorf("intl-messages.mjs lacks %s:\n%s", want, js)
		}
	}

	// A language just added to charter.toml's [intl] gets its catalog.
	if err := a.project("en", "en-IN", "de", "fr").Generate(nil); err != nil {
		t.Fatal(err)
	}
	if got := a.read("cat/fr.toml"); !strings.HasPrefix(got, "# fr: the translations of en.toml") {
		t.Errorf("fr.toml: %q", got)
	}

	// A key the source no longer has is obsolete, for charter intl prune.
	a.write("cat/de.toml", "hello = \"Hallo, {name}\"\nbye = \"Tschüss\"\n")
	if de := a.project("en", "de").Locale("de"); !slices.Equal(de.Obsolete, []string{"bye"}) {
		t.Errorf("obsolete = %v", de.Obsolete)
	}
	if err := catalog.RemoveMessages(filepath.Join(a.root, "cat/de.toml"), []string{"bye"}); err != nil {
		t.Fatal(err)
	}
	if got := a.read("cat/de.toml"); got != "hello = \"Hallo, {name}\"\n" {
		t.Errorf("pruned de.toml: %q", got)
	}
	// A key's comment goes with it, and so does a table left empty, with its header's comment; the next table's
	// comment stays, and a table that was empty before stays too.
	a.write("cat/de.toml", "hello = \"Hallo\"\n\n[old]\n# A key.\none = \"1\"\ntwo = \"2\"\n\n# The nav.\n[nav]\n# Gone.\nx = \"x\"\nhome = \"Start\"\n\n[kept]\n\n# Last.\n[last]\ny = \"y\"\n")
	if err := catalog.RemoveMessages(filepath.Join(a.root, "cat/de.toml"), []string{"old.one", "old.two", "nav.x", "last.y"}); err != nil {
		t.Fatal(err)
	}
	if got, want := a.read("cat/de.toml"), "hello = \"Hallo\"\n\n# The nav.\n[nav]\nhome = \"Start\"\n\n[kept]\n"; got != want {
		t.Errorf("pruned de.toml:\n%s\nwant:\n%s", got, want)
	}

	// A translation must use the source's arguments, with the same kinds.
	for _, bad := range []string{"hello = \"Hallo, {who}\"\n", "hello = \"Hallo, {name, number}\"\n"} {
		a.write("cat/de.toml", bad)
		if _, err := catalog.Read(config.Config{Locales: []string{"en", "de"}, Catalogs: "cat", Root: a.root}); err == nil {
			t.Errorf("de.toml %q: Read succeeded, want an error", bad)
		}
	}
}

// TestStale: a translation made before its source changed is stale until it is updated or confirmed.
func TestStale(t *testing.T) {
	a := newApp(t)
	a.write("cat/en.toml", "hello = \"Hello, {name}\"\nbye = \"Bye\"\n")
	a.write("cat/de.toml", "hello = \"Hallo, {name}\"\nbye = \"Tschüss\"\n")
	if err := a.project("en", "de").Generate(nil); err != nil {
		t.Fatal(err)
	}
	if sum := a.read("cat/intl.sum"); !strings.Contains(sum, "de bye ") || !strings.Contains(sum, "de hello ") {
		t.Fatalf("intl.sum:\n%s", sum)
	}

	a.write("cat/en.toml", "hello = \"Hi, {name}!\"\nbye = \"Goodbye\"\n")
	p := a.project("en", "de")
	de := p.Locale("de")
	if de.Count(catalog.Stale) != 2 || de.Current() || !de.Complete() {
		t.Fatalf("de: %s", de.Summary(2))
	}
	todo := de.Todo(p)
	if len(todo) != 2 || todo[0].Key != "bye" || todo[0].Translation != "Tschüss" || todo[0].State != catalog.Stale {
		t.Fatalf("todo: %+v", todo)
	}
	if !p.SumCurrent() {
		t.Error("intl.sum changed although no translation did")
	}

	// Updating a translation makes it current; confirming keeps one as it is.
	a.write("cat/de.toml", "hello = \"Hallo, {name}!\"\nbye = \"Tschüss\"\n")
	p = a.project("en", "de")
	if n, err := p.Confirm(p.Locale("de"), "bye"); err != nil || n != 1 {
		t.Fatalf("Confirm = %d, %v", n, err)
	}
	if err := p.Generate(nil); err != nil {
		t.Fatal(err)
	}
	if de := a.project("en", "de").Locale("de"); !de.Current() {
		t.Errorf("de after update and confirm: %s", de.Summary(2))
	}
}

// TestSetMessages: catalogs are edited in place: comments, order and other keys stay.
func TestSetMessages(t *testing.T) {
	a := newApp(t)
	a.write("cat/de.toml", "# Deutsch\ntitle = \"Alt\"\n\n# The board.\n[board]\nnotes = \"x\"\nlong = \"\"\"\nzwei\nZeilen\"\"\"\n\n[nav]\nhome = \"Start\"\n")
	if err := catalog.SetMessages(filepath.Join(a.root, "cat/de.toml"), map[string]string{
		"title": "Neu", "board.long": "eine", "board.post": "Senden \"jetzt\"", "nav.about": "Über", "help.faq": "FAQ",
	}); err != nil {
		t.Fatal(err)
	}
	// A dotted key inside a table keeps its name, and a header's comment isn't part of the table's name.
	a.write("cat/fr.toml", "[board] # the board\nnotes.title = \"Vieux\"\n")
	if err := catalog.SetMessages(filepath.Join(a.root, "cat/fr.toml"), map[string]string{"board.notes.title": "Neuf", "board.post": "Envoyer"}); err != nil {
		t.Fatal(err)
	}
	if got := a.read("cat/fr.toml"); got != "[board] # the board\nnotes.title = \"Neuf\"\npost = \"Envoyer\"\n" {
		t.Errorf("fr.toml:\n%s", got)
	}
	want := "# Deutsch\ntitle = \"Neu\"\n\n# The board.\n[board]\nnotes = \"x\"\nlong = \"eine\"\npost = \"Senden \\\"jetzt\\\"\"\n\n[nav]\nhome = \"Start\"\nabout = \"Über\"\n\n[help]\nfaq = \"FAQ\"\n"
	if got := a.read("cat/de.toml"); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestAddMessages: only keys the catalog doesn't have are written, after the last key of their table (not under the
// next table's comment), with their comments; a new table goes at the end under its own.
func TestAddMessages(t *testing.T) {
	a := newApp(t)
	a.write("cat/en.toml", "# The app.\n\n[intl]\nlanguages = \"Tongues\"\n\n# The board.\n[board]\nnotes = \"Notes\"\n")
	added, err := catalog.AddMessages(filepath.Join(a.root, "cat/en.toml"), []catalog.NewMessage{
		{Key: "intl.languages", Text: "Languages"},
		{Key: "intl.no_thanks", Text: "No thanks", Note: []string{"Turns the offer down."}},
		{Key: "gsxui.close", Text: "Close"},
	}, map[string][]string{"gsxui": {"gsxui's own text."}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(added, []string{"intl.no_thanks", "gsxui.close"}) {
		t.Errorf("added %v", added)
	}
	want := "# The app.\n\n[intl]\nlanguages = \"Tongues\"\n# Turns the offer down.\nno_thanks = \"No thanks\"\n\n# The board.\n[board]\nnotes = \"Notes\"\n\n# gsxui's own text.\n[gsxui]\nclose = \"Close\"\n"
	if got := a.read("cat/en.toml"); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if added, err := catalog.AddMessages(filepath.Join(a.root, "cat/en.toml"), []catalog.NewMessage{{Key: "intl.languages", Text: "x"}}, nil); err != nil || added != nil {
		t.Errorf("again: %v, %v", added, err)
	}
}

// TestCopiedFiles: intl.sum keeps the files charter intl add wrote through gen.
func TestCopiedFiles(t *testing.T) {
	a := newApp(t)
	a.write("cat/en.toml", "hi = \"Hi\"\n")
	a.write("cat/de.toml", "hi = \"Hallo\"\n")
	p := a.project("en", "de")
	p.SetCopied("views/intl/language-list.gsx", catalog.FileHash([]byte("x")))
	if err := p.WriteSum(); err != nil {
		t.Fatal(err)
	}
	p = a.project("en", "de")
	if err := p.WriteSum(); err != nil {
		t.Fatal(err)
	}
	if got := p.Copied("views/intl/language-list.gsx"); got != catalog.FileHash([]byte("x")) {
		t.Errorf("Copied = %q", got)
	}
	if s := a.read("cat/intl.sum"); !strings.HasSuffix(s, "de hi 3639efcd08ab 753692ec36ad\nfile views/intl/language-list.gsx 2d711642b726\n") {
		t.Errorf("intl.sum:\n%s", s)
	}
}

// TestXLIFF: what is left to translate goes out as XLIFF 2.0 and comes back into the catalog, checked.
func TestXLIFF(t *testing.T) {
	a := newApp(t)
	a.write("cat/en.toml", "# Greets the reader by name.\nhello = \"Hello, {name}\"\nbye = \"Bye\"\n")
	a.write("cat/de.toml", "bye = \"Tschüss\"\n")
	p := a.project("en", "de")
	b, n, err := p.ExportXLIFF(p.Locale("de"), false, "")
	if err != nil || n != 1 {
		t.Fatalf("export: %d, %v", n, err)
	}
	x := string(b)
	for _, want := range []string{`<xliff xmlns="urn:oasis:names:tc:xliff:document:2.0" version="2.0" srcLang="en" trgLang="de">`, `<unit id="hello">`,
		`<note category="context">Greets the reader by name.</note>`, `<segment state="initial">`, `<source>Hello, {name}</source>`} {
		if !strings.Contains(x, want) {
			t.Errorf("export lacks %s:\n%s", want, x)
		}
	}
	if strings.Contains(x, `id="bye"`) {
		t.Errorf("export has a translated message:\n%s", x)
	}

	bad := strings.Replace(x, `<segment state="initial">`, `<segment state="translated">`, 1)
	bad = strings.Replace(bad, "</source>", "</source>\n<target>Hallo, {wer}</target>", 1)
	if _, _, _, err := p.ImportXLIFF([]byte(bad)); err == nil || !strings.Contains(err.Error(), "{wer}") {
		t.Errorf("import of a wrong argument: %v", err)
	}
	good := strings.Replace(bad, "{wer}", "{name}", 1)
	if l, changed, _, err := p.ImportXLIFF([]byte(good)); err != nil || changed != 1 || l.ID != "de" {
		t.Fatalf("import: %v %d %v", l, changed, err)
	}
	if de := a.project("en", "de").Locale("de"); !de.Complete() {
		t.Errorf("de after import: %s", de.Summary(2))
	}
}

// TestUnused: a message no Go or gsx file names is unused.
func TestUnused(t *testing.T) {
	a := newApp(t)
	a.write("cat/en.toml", "used = \"A\"\nviewed = \"B {n, number}\"\ndead = \"C\"\n")
	a.write("main.go", "package main\n\nvar _ = cat.Used\n")
	a.write("views/page.gsx", "<p>{ cat.Viewed(3) }</p>\n")
	a.write("nested/go.mod", "module example.com/other\n")
	a.write("nested/x.go", "var _ = cat.Dead\n")
	unused, err := a.project("en").Unused()
	if err != nil || !slices.Equal(unused, []string{"dead"}) {
		t.Errorf("Unused = %v, %v", unused, err)
	}
}

// TestContext: comments above a key and its table reach translators.
func TestContext(t *testing.T) {
	a := newApp(t)
	a.write("cat/en.toml", "# A heading.\ntitle = \"Board\"\n\n# The board page.\n[board]\n# How many notes it shows.\nnotes = \"{n, plural, one {# note} other {# notes}}\"\npost = \"Post\"\n")
	p := a.project("en")
	for key, want := range map[string]string{"title": "A heading.", "board.notes": "The board page. How many notes it shows.", "board.post": "The board page."} {
		if got := p.Message(key).Context; got != want {
			t.Errorf("%s context = %q, want %q", key, got, want)
		}
	}
}

// TestAddLocale: a language goes into charter.toml's [intl] and gets a catalog.
func TestAddLocale(t *testing.T) {
	a := newApp(t)
	a.write("charter.toml", "# the app's languages\n[intl]\nlocales = [\"en\", \"de\"]\ncatalogs = \"cat\"\n")
	a.write("cat/en.toml", "hi = \"Hi\"\n")
	c, err := config.Find(a.root)
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.AddLocale(c, "fr-CA", "français (Canada)"); err != nil {
		t.Fatal(err)
	}
	if got := a.read("charter.toml"); !strings.Contains(got, `locales = ["en", "de", "fr-CA"]`) || !strings.HasPrefix(got, "# the app's languages\n") {
		t.Errorf("charter.toml's [intl]:\n%s", got)
	}
	if got := a.read("cat/fr-CA.toml"); !strings.HasPrefix(got, "# français (Canada) (fr-CA): the translations of en.toml") {
		t.Errorf("fr-CA.toml:\n%s", got)
	}
	if err := catalog.AddLocale(c, "de", "Deutsch"); err == nil {
		t.Error("adding a shipped locale succeeded")
	}
}

// TestGsxui: the text vendored gsxui components write (i18n.T) is translated from the catalogs' [gsxui] messages, and
// a T without one stops gen, naming the key to add.
func TestGsxui(t *testing.T) {
	a := newApp(t)
	a.write("ui/i18n/i18n.go", "package i18n\n\ntype T string\n")
	a.write("cat/en.toml", "[gsxui]\nclose = \"Close\"\nnext_slide = \"Next slide\"\n")
	a.write("ui/dialog.gsx", "<span class=\"sr-only\">{ i18n.T(\"Close\") }</span>\n")
	a.write("ui/carousel.gsx", "<span>{ i18n.T(\"Next slide\") }</span><b aria-label={ i18n.T(\"Close\") }/>\n")
	if err := a.project("en").Generate(nil); err != nil {
		t.Fatal(err)
	}
	b := a.read("ui/i18n/intl_gen.go")
	for _, want := range []string{"package i18n", `"example.com/app/cat"`, "func Translate(ctx context.Context, m T) string {",
		"case \"Close\":\n\t\treturn intl.Translate(ctx, cat.GsxuiClose)", "case \"Next slide\":\n\t\treturn intl.Translate(ctx, cat.GsxuiNextSlide)"} {
		if !strings.Contains(b, want) {
			t.Errorf("ui/i18n/intl_gen.go lacks %q:\n%s", want, b)
		}
	}

	// gsxui add pagination: its "More pages" has no message yet.
	a.write("ui/pagination.gsx", "<span>{ i18n.T(\"More pages\") }</span>\n")
	err := a.project("en").Generate(nil)
	if want := `ui/pagination.gsx writes i18n.T("More pages"): add gsxui.more_pages = "More pages" to cat/en.toml`; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Generate = %v, want …%s", err, want)
	}

	// A placeholder gsxui's own script fills in is quoted, so ICU writes it as it is.
	a.write("ui/pagination.gsx", "")
	a.write("ui/calendar.gsx", "<b data-label={ i18n.T(\"Today, {date}\") }/>\n")
	err = a.project("en").Generate(nil)
	if want := `add gsxui.today_date = "Today, '{date}'"`; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Generate = %v, want …%s", err, want)
	}
}

// TestBadge: [intl] badge, written by gen from the catalogs: how many languages, and whether all are translated.
func TestBadge(t *testing.T) {
	a := newApp(t)
	a.write("cat/en.toml", "hello = \"Hello\"\n")
	a.write("cat/de.toml", "hello = \"Hallo\"\n")
	project := func() *catalog.Project {
		p, err := catalog.Read(config.Config{Locales: []string{"en", "de", "fr"}, Catalogs: "cat", Root: a.root, Badge: "docs/badges/intl.svg"})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	a.write("cat/fr.toml", "")
	if err := project().Generate(nil); err != nil {
		t.Fatal(err)
	}
	if svg := a.read("docs/badges/intl.svg"); !strings.Contains(svg, "languages: 3, 1 not translated") {
		t.Errorf("badge with fr missing:\n%s", svg)
	}
	a.write("cat/fr.toml", "hello = \"Bonjour\"\n")
	if project().BadgeCurrent() {
		t.Error("BadgeCurrent after fr was translated: want false until gen")
	}
	if err := project().Generate(nil); err != nil {
		t.Fatal(err)
	}
	if svg := a.read("docs/badges/intl.svg"); !strings.Contains(svg, "languages: 3, all translated") || !project().BadgeCurrent() {
		t.Errorf("badge with every language translated:\n%s", svg)
	}
}
