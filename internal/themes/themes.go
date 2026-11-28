// Package themes writes every theme gsxui has for an app's style and base colour into one stylesheet, so that a
// reader can choose among them as the page runs.
//
// gsxui's own design: a style (vega, nova, maia, lyra, mira, luma, sera, rhea) is compiled into the components,
// one for an app, and is changed while developing (gsxui apply; charter run ui:theme). A theme changes CSS
// variables only, and nothing else: so a theme can be changed in the browser, light and dark alike. gsxui installs
// one theme (web/gsxui/theme.css). This asks gsxui itself for each of the others, the accents of its theme
// editor (https://ui.gsxhq.dev/theme: amber, blue … yellow), and keeps what each changes:
//
//	[data-theme="blue"] { --primary: …; --sidebar-primary: …; … }
//	.dark[data-theme="blue"], .dark [data-theme="blue"] { … }
//
// Nothing here knows a colour. Each theme is resolved by gsxui's own code: gsxui apply, run on a copy of the
// app's gsxui files under .charter/, with the theme's share code. A share code is how gsxui names a combination
// of its choices (gsxui:p1:…), and what apply takes; its layout is gsxui's, and is repeated below because gsxui
// has no command that makes one from names. A test holds the copy to gsxui's own source.
//
// The file is web/gsxui/themes.css, committed, and written by charter check --fix when the app's preset or its
// gsxui has changed (its first line says from which); charter check fails while it is behind.
package themes

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joeblew999/charter/internal/gsxuinames"
)

// gsxui's share code, as its internal/preset/compact.go has it ("the append-only p1 transport ABI"): the index of
// each choice in these lists, packed into one number, in base 62.
var (
	Styles = gsxuinames.Styles
	Bases  = gsxuinames.Bases
	// A theme is a base colour's own, or one of the accents after them.
	all = slices.Concat(gsxuinames.Bases, gsxuinames.Accents)
)

const (
	prefix      = "gsxui:p1:"
	alphabet    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	baseShift   = 4  // after the style's 4 bits
	themeShift  = 8  // after the base colour's 4
	radiusShift = 13 // after the theme's 5
	// The radius every code here has: "medium", gsxui's default. Only what a theme changes is kept, so which
	// radius, menu accent, chart colour and fonts a code says does not matter, as long as all say the same.
	radiusMedium = 2
)

// Accents are the themes a reader may choose beside the app's own: gsxui's accents, in gsxui's order.
func Accents() []string { return slices.Clone(gsxuinames.Accents) }

// Code is gsxui's share code for a style, a base colour and a theme.
func Code(style, base, theme string) (string, error) {
	s, b, t := slices.Index(Styles, style), slices.Index(Bases, base), slices.Index(all, theme)
	if s < 0 || b < 0 || t < 0 {
		return "", fmt.Errorf("gsxui has no style %q, base colour %q or theme %q", style, base, theme)
	}
	packed := uint64(s) | uint64(b)<<baseShift | uint64(t)<<themeShift | uint64(radiusMedium)<<radiusShift
	code := ""
	for ; packed > 0; packed /= 62 {
		code = string(alphabet[packed%62]) + code
	}
	if code == "" {
		code = "0"
	}
	return prefix + code, nil
}

// File is the stylesheet, in an app.
var File = filepath.Join("web", "gsxui", "themes.css")

// values are a theme's variables for light and for dark, as gsxui.preset.json has them.
type values struct {
	Light map[string]string `json:"light"`
	Dark  map[string]string `json:"dark"`
}

type preset struct {
	Style string `json:"style"`
	Theme values `json:"theme"`
}

func read(file string) (preset, []byte, error) {
	var p preset
	b, err := os.ReadFile(file)
	if err != nil {
		return p, nil, err
	}
	return p, b, json.Unmarshal(b, &p)
}

var keyed = regexp.MustCompile(`from ([0-9a-f]{16})\b`)

// Write writes dir's web/gsxui/themes.css when it is not what the app's preset and its gsxui give; with check it
// writes nothing and fails then. An app with no gsxui has none.
func Write(dir string, check bool) error {
	app, raw, err := read(filepath.Join(dir, "gsxui.preset.json"))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("gsxui.preset.json: %w", err)
	}
	version, err := exec.Command("go", "list", "-C", dir, "-m", "-f", "{{.Version}}{{.Dir}}", "github.com/gsxhq/gsxui").Output()
	if err != nil {
		return fmt.Errorf("the app's gsxui: %w", err)
	}
	key := fmt.Sprintf("%x", sha256.Sum256(append(version, raw...)))[:16]
	file := filepath.Join(dir, File)
	if old, err := os.ReadFile(file); err == nil {
		if m := keyed.FindSubmatch(old); m != nil && string(m[1]) == key {
			return nil
		}
	}
	if check {
		return fmt.Errorf("%s is not what the app's preset (gsxui.preset.json) and its gsxui give: charter check --fix writes it", filepath.ToSlash(File))
	}
	css, err := resolve(dir, app, key)
	if err != nil {
		return err
	}
	fmt.Printf("%s: gsxui's %d themes for the app's style and base colour\n", filepath.ToSlash(File), len(Accents()))
	return os.WriteFile(file, []byte(css), 0o644)
}

// resolve asks gsxui for each theme and gives the stylesheet.
func resolve(dir string, app preset, key string) (string, error) {
	if !slices.Contains(Styles, app.Style) {
		return "", fmt.Errorf("gsxui.preset.json: the style %q is none of gsxui's (%s)", app.Style, strings.Join(Styles, ", "))
	}
	scratch := filepath.Join(dir, ".charter", "themes")
	os.RemoveAll(scratch)
	defer os.RemoveAll(scratch)
	if err := copied(dir, scratch); err != nil {
		return "", err
	}
	bin, err := exec.Command("go", "tool", "-C", dir, "-n", "gsxui").Output()
	if err != nil {
		return "", fmt.Errorf("the app's gsxui (go tool gsxui): %w", err)
	}
	apply := func(base, theme string) (values, error) {
		code, err := Code(app.Style, base, theme)
		if err != nil {
			return values{}, err
		}
		cmd := exec.Command(strings.TrimSpace(string(bin)), "apply", "--only", "theme", "--preset", code, "--yes")
		cmd.Dir = scratch
		if out, err := cmd.CombinedOutput(); err != nil {
			return values{}, fmt.Errorf("gsxui apply --only theme --preset %s: %w\n%s", code, err, out)
		}
		got, _, err := read(filepath.Join(scratch, "gsxui.preset.json"))
		return got.Theme, err
	}
	// The app's base colour: the one whose own theme has the app's page and its lines. An app with colours of its
	// own has none of gsxui's, and takes the accents as they are on the first.
	base, plain := Bases[0], values{}
	for i, candidate := range Bases {
		own, err := apply(candidate, candidate)
		if err != nil {
			return "", err
		}
		if i == 0 {
			plain = own
		}
		if same(own, app.Theme, "background", "foreground", "border", "muted", "muted-foreground") {
			base, plain = candidate, own
			break
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "/* Written by charter check --fix from %s: gsxui's own themes for this app's style (%s) and base colour (%s),\n", key, app.Style, base)
	out.WriteString("   each resolved by gsxui apply, and of each only what it changes. Do not edit: change gsxui.preset.json\n")
	out.WriteString("   (charter run ui:theme), and charter check --fix writes this again. <html data-theme=\"blue\"> has a theme;\n")
	out.WriteString("   without one the page has the app's own (theme.css). */\n")
	for _, accent := range Accents() {
		themed, err := apply(base, accent)
		if err != nil {
			return "", err
		}
		light := changed(plain.Light, themed.Light)
		block(&out, fmt.Sprintf(`[data-theme=%q]`, accent), light)
		block(&out, fmt.Sprintf(`.dark[data-theme=%q], .dark [data-theme=%q]`, accent, accent), dark(light, plain.Dark, themed.Dark))
	}
	// Which choice is the one in force is said by the stylesheet too: whatever lists the themes marks nothing itself.
	out.WriteString("\n/* The mark of the choice in force, in whatever lists them ([data-theme-choice], with a [data-theme-chosen] in it). */\n")
	out.WriteString("[data-theme-chosen] { visibility: hidden; }\n")
	out.WriteString(":root:not([data-theme]) [data-theme-choice=\"\"] [data-theme-chosen]")
	for _, accent := range Accents() {
		fmt.Fprintf(&out, ",\n:root[data-theme=%q] [data-theme-choice=%q] [data-theme-chosen]", accent, accent)
	}
	out.WriteString(" { visibility: visible; }\n")
	return out.String(), nil
}

// same reports whether two themes have the same values for these variables, light and dark.
func same(a, b values, names ...string) bool {
	for _, name := range names {
		if a.Light[name] != b.Light[name] || a.Dark[name] != b.Dark[name] {
			return false
		}
	}
	return true
}

// changed are the variables a theme has otherwise than the plain one, by name.
func changed(plain, themed map[string]string) [][2]string {
	var out [][2]string
	for _, name := range slices.Sorted(keys(themed)) {
		if themed[name] != plain[name] {
			out = append(out, [2]string{name, themed[name]})
		}
	}
	return out
}

// dark are the variables a theme has in the dark: what it changes there, and every one it changes in the light,
// with its dark value, changed or not. The light ones are set on the same element, and would otherwise still hold
// in the dark, where the plain theme's own were meant.
func dark(light [][2]string, plain, themed map[string]string) [][2]string {
	out := changed(plain, themed)
	for _, v := range light {
		if !slices.ContainsFunc(out, func(d [2]string) bool { return d[0] == v[0] }) {
			if value, ok := themed[v[0]]; ok {
				out = append(out, [2]string{v[0], value})
			}
		}
	}
	slices.SortFunc(out, func(a, b [2]string) int { return strings.Compare(a[0], b[0]) })
	return out
}

func keys(m map[string]string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func block(out *strings.Builder, selector string, variables [][2]string) {
	if len(variables) == 0 {
		return
	}
	fmt.Fprintf(out, "\n%s {\n", selector)
	for _, v := range variables {
		fmt.Fprintf(out, "  --%s: %s;\n", v[0], v[1])
	}
	out.WriteString("}\n")
}

// copied makes scratch a place gsxui apply can run in for the app in dir: its gsxui files, and its go.mod with
// the modules it replaces by a folder beside it found from there too.
func copied(dir, scratch string) error {
	if err := os.MkdirAll(filepath.Join(scratch, "web", "gsxui"), 0o755); err != nil {
		return err
	}
	files, _ := filepath.Glob(filepath.Join(dir, "web", "gsxui", "*.css"))
	for _, name := range []string{"gsxui.json", "gsxui.preset.json", "go.sum"} {
		files = append(files, filepath.Join(dir, name))
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, file)
		if err := os.WriteFile(filepath.Join(scratch, rel), b, 0o644); err != nil {
			return err
		}
	}
	mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	beside := regexp.MustCompile(`(?m)(=>\s*)(\.{1,2}(?:/\S*)?)\s*$`)
	mod = beside.ReplaceAllFunc(mod, func(m []byte) []byte {
		parts := beside.FindSubmatch(m)
		return append(slices.Clone(parts[1]), filepath.Join(abs, string(parts[2]))...)
	})
	if err := os.WriteFile(filepath.Join(scratch, "go.mod"), mod, 0o644); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(scratch, "gsxui.json")); err != nil {
		return errors.New("the app has no gsxui.json: go tool gsxui init")
	}
	return nil
}
