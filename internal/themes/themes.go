// Package themes brings gsxui's theme editor (https://ui.gsxhq.dev/theme) into an app, as gsxui has it: its
// markup, its scripts and its catalogue of styles, base colours, themes, chart colours, menu accents, radii and
// fonts. Nothing of it is charter's: a reader of the app chooses with gsxui's own controls, the app's own pages
// are the editor's live preview by gsxui's own preview protocol, and what is chosen is gsxui's preset, kept and
// shared as gsxui's share code.
//
// gsxui has the editor as Go in its own module (site/pages: ThemeEditor, "the editor body without the site
// shell") and as three scripts (web/theme.js, web/theme-state.js, web/theme-preview.js). Compiled into a Worker
// the editor is 25 MiB and its preview of every component in every style 40, of the 64 a Worker may have. So it
// is rendered here, once, on this machine, by a program that calls gsxui's ThemeEditor, and what comes out (the
// markup, with the catalogue in it) is a file of the app; the scripts are copied beside it, as gsxui copies its
// own into web/gsxui. The page that shows it is the theme screen.
//
// A style (vega, nova, maia, lyra, mira, luma, sera, rhea) is the one part of a preset that is markup and not CSS:
// the app has its components in each (styles.go), and the editor tells the Worker which a reader chose.
//
// Written by charter check --fix when the app's preset, its gsxui or this package has changed (the generated Go
// says from which); charter check fails while any of it is behind.
package themes

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
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

// Where gsxui's editor is in an app: its scripts as gsxui has them and what connects them to the app, and the
// editor's markup with the Go that hands it to the theme screen.
var (
	scripts = filepath.Join("web", "gsxui-site")
	markup  = filepath.Join("pages", "screens", "theme-editor.gen.html")
	handed  = filepath.Join("pages", "screens", "theme_editor.gen.go")
)

// PreviewPlaceholder stands in the markup for the address of the page the editor previews, which the theme screen
// knows when it is asked for (the app's front page, in the reader's language).
const PreviewPlaceholder = "/__charter/theme-preview-address"

//go:embed js/charter.js
var connection string

var keyed = regexp.MustCompile(`from ([0-9a-f]{16})\b`)

// gsxuiScripts are the editor's scripts, in gsxui's web folder.
var gsxuiScripts = []string{"theme.js", "theme-state.js", "theme-preview.js"}

// Write writes gsxui's theme editor into the app in dir when what is there is not what the app's preset, its gsxui
// and this package give; with check it writes nothing and fails then. An app with no gsxui has none.
func Write(dir string, check bool) error {
	preset, err := os.ReadFile(filepath.Join(dir, "gsxui.preset.json"))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	out, err := exec.Command("go", "list", "-C", dir, "-m", "-f", "{{.Version}}\n{{.Dir}}", "github.com/gsxhq/gsxui").Output()
	if err != nil {
		return fmt.Errorf("the app's gsxui: %w", err)
	}
	version, gsxui, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(version+"\x00"+string(preset)+"\x00"+connection)))[:16]
	if old, err := os.ReadFile(filepath.Join(dir, handed)); err == nil {
		if m := keyed.FindSubmatch(old); m != nil && string(m[1]) == key && present(dir) {
			return nil
		}
	}
	if check {
		return fmt.Errorf("gsxui's theme editor in the app (%s, %s) is not what the app's preset and its gsxui give: charter check --fix writes it", filepath.ToSlash(scripts), filepath.ToSlash(markup))
	}
	// gsxui's scripts, as they are, and beside them what connects them to the app. gsxui's editor script imports
	// gsxui's own runtime from ../ui/gsxui.js: here that is the app's (web/gsxui/gsxui.js), so there is one.
	files := map[string]string{
		filepath.Join(scripts, "charter.js"):     connection,
		filepath.Join(scripts, "ui", "gsxui.js"): "// gsxui's editor imports gsxui's runtime from here: it is the app's own (web/gsxui/gsxui.js). Written by charter check --fix.\nexport * from \"../../gsxui/gsxui.js\";\n",
	}
	for _, name := range gsxuiScripts {
		b, err := os.ReadFile(filepath.Join(gsxui, "web", name))
		if err != nil {
			return fmt.Errorf("gsxui's theme editor is no longer where it was (web/%s): %w", name, err)
		}
		files[filepath.Join(scripts, "web", name)] = string(b)
	}
	sheet, err := docsSheet(gsxui)
	if err != nil {
		return err
	}
	files[filepath.Join(scripts, siteSheet)] = sheet
	html, err := rendered(dir)
	if err != nil {
		return err
	}
	files[markup] = html
	files[handed] = "// Code generated by charter check --fix from " + key + ": gsxui's theme editor (its markup, with its catalogue) and the\n" +
		"// app's own preset, for the theme screen. DO NOT EDIT: change gsxui.preset.json (charter run ui:theme).\n\n" +
		"package screens\n\nimport (\n\t_ \"embed\"\n\n\t\"github.com/joeblew999/charter/app\"\n)\n\n" +
		"//go:embed theme-editor.gen.html\nvar themeEditorMarkup string\n\n" +
		"func init() { app.SetThemeEditor(themeEditorMarkup, " + fmt.Sprintf("%q", compact(preset)) + ") }\n"
	for rel, text := range files {
		file := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("%s, %s: gsxui's theme editor, from gsxui %s\n", filepath.ToSlash(scripts), filepath.ToSlash(markup), version)
	return needs(dir, gsxui)
}

// present reports whether every file Write writes is there.
func present(dir string) bool {
	names := []string{markup, filepath.Join(scripts, "charter.js"), filepath.Join(scripts, "ui", "gsxui.js"), filepath.Join(scripts, siteSheet)}
	for _, name := range gsxuiScripts {
		names = append(names, filepath.Join(scripts, "web", name))
	}
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}
	return true
}

// siteSheet is gsxui's own stylesheet for its site's docs, in the app.
const siteSheet = "site.css"

// What of gsxui's web/site.css is an app's: where its doc pages' look begins, and where what colours their code
// begins, which runs to the file's end. Between the two and before them is what is gsxui's site's alone (its
// fonts, its logo, where Tailwind reads its own templates).
const (
	proseStart = "@layer base {\n  /* Doc prose"
	codeStart  = "/* Syntax highlighting for docs code blocks."
)

// docsSheet is how gsxui's own site's docs look, taken from gsxui's stylesheet for its site (web/site.css) as it
// is: the rules for a container marked data-doc (headings, paragraphs, links, code) and the colours of a sample's
// words. A docs page of the app is such a container (the docs screen), so it looks as gsxui's docs do, in the
// app's theme. Nothing in it is charter's but the first line, which lets Tailwind read gsxui's own names in it.
func docsSheet(gsxui string) (string, error) {
	b, err := os.ReadFile(filepath.Join(gsxui, "web", "site.css"))
	if err != nil {
		return "", fmt.Errorf("gsxui's stylesheet for its site is no longer where it was (web/site.css): %w", err)
	}
	text := string(b)
	prose, code := strings.Index(text, proseStart), strings.Index(text, codeStart)
	if prose < 0 || code < prose {
		return "", errors.New("gsxui's stylesheet for its site (web/site.css) no longer has its docs' look where it was (\"Doc prose\", \"Syntax highlighting for docs code blocks\"): this charter does not know this gsxui")
	}
	return "/* gsxui's own stylesheet for its site's docs (web/site.css in gsxui), from where its doc pages' look begins: written\n   by charter check --fix, not to be edited. A docs page of the app is marked data-doc, as gsxui's are. */\n@reference \"../gsxui/index.css\";\n\n" + text[prose:], nil
}

// compact is JSON on one line.
func compact(raw []byte) string {
	var out bytes.Buffer
	if json.Compact(&out, raw) != nil {
		return string(raw)
	}
	return out.String()
}

// rendered is gsxui's theme editor as gsxui's own Go renders it: a program of the app's module, run on this
// machine, that calls gsxui's ThemeEditor. The catalogue is in the markup, as gsxui puts it there for its script.
func rendered(dir string) (string, error) {
	program := filepath.Join(dir, ".charter", "themeeditor")
	if err := os.MkdirAll(program, 0o755); err != nil {
		return "", err
	}
	defer os.RemoveAll(program)
	main := "// Written by charter check --fix, each time it runs: gsxui's theme editor, rendered.\npackage main\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t\"os\"\n\n\t\"github.com/gsxhq/gsxui/site/pages\"\n)\n\n" +
		"func main() {\n\tif err := pages.ThemeEditor(" + fmt.Sprintf("%q", PreviewPlaceholder) + ").Render(context.Background(), os.Stdout); err != nil {\n\t\tfmt.Fprintln(os.Stderr, err)\n\t\tos.Exit(1)\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(program, "main.go"), []byte(main), 0o644); err != nil {
		return "", err
	}
	cmd := exec.Command("go", "run", "./.charter/themeeditor")
	cmd.Dir = dir
	var out, said bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &said
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("rendering gsxui's theme editor (github.com/gsxhq/gsxui/site/pages.ThemeEditor): %w\n%s", err, said.String())
	}
	if !strings.Contains(out.String(), "data-theme-schema") || !strings.Contains(out.String(), PreviewPlaceholder) {
		return "", errors.New("gsxui's theme editor no longer has its catalogue or its preview where they were (data-theme-schema, data-theme-preview-frame): this charter does not know this gsxui")
	}
	return out.String(), nil
}

// needs has the app's npm packages include what gsxui's editor scripts import, at the versions gsxui has them.
func needs(dir, gsxui string) error {
	var theirs, ours struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	b, err := os.ReadFile(filepath.Join(gsxui, "package.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &theirs); err != nil {
		return err
	}
	if b, err = os.ReadFile(filepath.Join(dir, "package.json")); err != nil {
		return err
	}
	if err := json.Unmarshal(b, &ours); err != nil {
		return err
	}
	var missing []string
	for _, name := range []string{"css-tree", "jsonc-parser"} {
		if ours.Dependencies[name] == "" && ours.DevDependencies[name] == "" {
			missing = append(missing, name+"@"+theirs.Dependencies[name])
		}
	}
	if len(missing) == 0 {
		return nil
	}
	fmt.Printf("package.json: %s, which gsxui's theme editor imports\n", strings.Join(missing, ", "))
	cmd := exec.Command("npm", append([]string{"install", "--save", "--no-audit", "--no-fund"}, missing...)...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stderr, os.Stderr
	return cmd.Run()
}
