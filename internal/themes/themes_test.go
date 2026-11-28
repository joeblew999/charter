package themes

import (
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/joeblew999/charter/internal/docsgen"
)

// The layout of a share code here is gsxui's own, read from gsxui's source as the module cache has it: its lists
// of styles, base colours and themes in their order, and where each sits in the number. When gsxui changes one,
// this fails before a theme is asked for by the wrong code.
func TestTheShareCodeIsGsxuis(t *testing.T) {
	dir, err := exec.Command("go", "list", "-C", filepath.Join("..", "..", "example"), "-m", "-f", "{{.Dir}}", "github.com/gsxhq/gsxui").Output()
	if err != nil {
		t.Skipf("the example's gsxui is not here: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), "internal", "preset", "compact.go"))
	if err != nil {
		t.Fatalf("gsxui's share code is no longer where it was (internal/preset/compact.go): %v", err)
	}
	source := string(b)
	list := func(name string) []string {
		m := regexp.MustCompile(`(?s)var ` + name + ` = \[\]\w+\{(.*?)\n\}`).FindStringSubmatch(source)
		if m == nil {
			t.Fatalf("gsxui's source has no %s", name)
		}
		var out []string
		for _, item := range regexp.MustCompile(`"([a-z-]+)"|Style(\w+),`).FindAllStringSubmatch(m[1], -1) {
			out = append(out, strings.ToLower(item[1]+item[2]))
		}
		return out
	}
	for name, ours := range map[string][]string{"compactStyles": Styles, "compactBaseColors": Bases, "compactThemes": all} {
		if theirs := list(name); strings.Join(theirs, " ") != strings.Join(ours, " ") {
			t.Errorf("%s is %v in gsxui, and %v here", name, theirs, ours)
		}
	}
	for _, want := range []string{`compactSharePrefix = "gsxui:p1:"`, "compactStyleBits       = 4", "compactBaseColorBits   = 4", "compactThemeBits       = 5", `"medium",`, `base62Alphabet     = "` + alphabet + `"`} {
		if !strings.Contains(source, want) {
			t.Errorf("gsxui's source no longer has %q: the share code's layout has changed", want)
		}
	}
}

// A code is the choices' places packed into one number in base 62: nova, neutral and its own theme at gsxui's
// default radius is the code gsxui itself gives for its default preset.
func TestCode(t *testing.T) {
	for _, c := range []struct{ style, base, theme, want string }{
		{"nova", "neutral", "neutral", "gsxui:p1:4GG"},
		{"nova", "neutral", "blue", "gsxui:p1:4nI"},
	} {
		if got, err := Code(c.style, c.base, c.theme); err != nil || got != c.want {
			t.Errorf("Code(%s, %s, %s) = %q, %v; want %q", c.style, c.base, c.theme, got, err, c.want)
		}
	}
	if _, err := Code("nova", "neutral", "teal-ish"); err == nil {
		t.Error("a theme gsxui has not was given a code")
	}
}

// What ui/styles has for a component: the same name and parameters, rendering the reader's style of it; and one
// that takes a type of its own package stays the app's own.
func TestChooser(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "thing.gsx")
	text := "package ui\n\ntype ThingConfig struct{}\n\ncomponent Thing(variant string, at time.Time, children gsx.Node, attrs gsx.Attrs) {\n}\n\ncomponent Own(config ThingConfig) {\n}\n\ncomponent hidden() {\n}\n\nfunc Axis(key string) gsx.Node {\n}\n\nfunc Width() string {\n}\n"
	if err := os.WriteFile(source, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := chooser(dir, "example.com/app", []string{source}, []string{"lyra", "maia"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"package ui\n",
		`own "example.com/app/ui"`,
		`"example.com/app/ui/styles/lyra"`,
		`"time"`,
		"type ThingConfig = own.ThingConfig\n",
		"func Thing(variant string, at time.Time, children gsx.Node, attrs gsx.Attrs) gsx.Node {",
		"\t\tcase \"maia\":\n\t\t\treturn maia.Thing(variant, at, children, attrs).Render(ctx, w)\n",
		"\t\treturn own.Thing(variant, at, children, attrs).Render(ctx, w)\n",
		"var Own = own.Own\n",
		"func Axis(key string) gsx.Node {",
		"var Width = own.Width\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ui/styles lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "hidden") {
		t.Errorf("ui/styles has a component of ui's that nothing outside it can name:\n%s", got)
	}
	if _, err := format.Source([]byte(got)); err != nil {
		t.Errorf("ui/styles is not Go: %v\n%s", err, got)
	}
}

// How a docs page looks is gsxui's own stylesheet for its site's docs, taken as it is: it styles a container marked
// data-doc, and colours the words of a sample under every name charter build gives one (docsgen.Names). When gsxui
// renames one, this fails before a page is shown with its code uncoloured.
func TestTheDocsLookIsGsxuis(t *testing.T) {
	dir, err := exec.Command("go", "list", "-C", filepath.Join("..", "..", "example"), "-m", "-f", "{{.Dir}}", "github.com/gsxhq/gsxui").Output()
	if err != nil {
		t.Skipf("the example's gsxui is not here: %v", err)
	}
	sheet, err := docsSheet(strings.TrimSpace(string(dir)))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[data-doc] h2", "[data-doc] pre", "[data-doc] :not(pre) > code", `@reference "../gsxui/index.css";`} {
		if !strings.Contains(sheet, want) {
			t.Errorf("gsxui's stylesheet for its docs has no %q", want)
		}
	}
	for _, name := range docsgen.Names {
		if !regexp.MustCompile(`code \.` + name + `[,\s{]`).MatchString(sheet) {
			t.Errorf("charter build names a word of a sample %s, which gsxui's stylesheet does not colour", name)
		}
	}
	for _, not := range []string{"--site-logo", "@source", "@fontsource"} {
		if strings.Contains(sheet, not) {
			t.Errorf("what is gsxui's site's alone is in the app's stylesheet: %s", not)
		}
	}
}
