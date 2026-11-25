package setup_test

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/joeblew999/charter/internal/intlkit/setup"
)

func app(t *testing.T, files map[string]string) string {
	root := t.TempDir()
	files["go.mod"] = "module example.com/shop\n"
	for name, s := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func read(t *testing.T, root, name string) string {
	b, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// gsxConfig is gsx.toml as gsx reads it.
type gsxConfig struct {
	ClassMerger    string            `toml:"class_merger"`
	FilterPackages []string          `toml:"filter_packages"`
	Renderers      map[string]string `toml:"renderers"`
	Dev            map[string]any    `toml:"dev"`
}

func TestInitFresh(t *testing.T) {
	root := app(t, map[string]string{})
	o := setup.Options{Locales: []string{"en", "de"}, Catalogs: "locales", Gsxui: "ui"}
	if err := setup.Init(root, o, io.Discard); err != nil {
		t.Fatal(err)
	}
	if s := read(t, root, "intl.toml"); !strings.Contains(s, `locales = ["en", "de"]`) || !strings.Contains(s, `gsxui = "ui"`) {
		t.Errorf("intl.toml:\n%s", s)
	}
	if s := read(t, root, "locales/en.toml"); !strings.Contains(s, `#   title = "Welcome"`) {
		t.Errorf("en.toml:\n%s", s)
	}
	var g gsxConfig
	if _, err := toml.Decode(read(t, root, "gsx.toml"), &g); err != nil {
		t.Fatal(err)
	}
	// gsxui's text renderer waits for gsxui's i18n package: a renderer for a package that isn't there breaks gsx.
	if len(g.FilterPackages) != 1 || g.Renderers["github.com/joeblew999/charter/site/intl.Msg"] != "github.com/joeblew999/charter/site/intl.Translate" ||
		g.Renderers["example.com/shop/ui/i18n.T"] != "" {
		t.Errorf("gsx.toml before gsxui's i18n: %+v", g)
	}
	// gsxui add vendors ui/i18n with the first component that writes text; the next charter intl gen registers it.
	if err := os.MkdirAll(filepath.Join(root, "ui", "i18n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ui", "i18n", "i18n.go"), []byte("package i18n\n\ntype T string\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := setup.GsxuiRenderer(root, "ui", io.Discard); err != nil {
		t.Fatal(err)
	}
	g = gsxConfig{}
	if _, err := toml.Decode(read(t, root, "gsx.toml"), &g); err != nil || g.Renderers["example.com/shop/ui/i18n.T"] != "example.com/shop/ui/i18n.Translate" {
		t.Errorf("gsx.toml with gsxui's i18n: %+v %v", g, err)
	}
	if s := read(t, root, ".gitignore"); !strings.Contains(s, "\nintl_gen.go\nintl-messages.mjs\nintl-workerd/\n") {
		t.Errorf(".gitignore:\n%s", s)
	}
	// No mise.toml: Go's version for mise is the app's own (the template's app has it, charter intl template), and
	// intl fetches the workerd it pins itself (intl/workerd.Path), so an app needs only its module.
	if _, err := os.Stat(filepath.Join(root, "mise.toml")); err == nil {
		t.Errorf("init wrote mise.toml:\n%s", read(t, root, "mise.toml"))
	}
}

// TestInitWithoutGsx: an app without gsx (no gsxui, no gsx.toml, no .gsx files) gets nothing of gsx's, even when its
// go.mod requires gsx for something else.
func TestInitWithoutGsx(t *testing.T) {
	root := app(t, map[string]string{"main.go": "package main\n"})
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/shop\n\nrequire github.com/gsxhq/gsx v0.5.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := setup.Init(root, setup.Options{Locales: []string{"en"}, Catalogs: "locales"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "gsx.toml")); err == nil {
		t.Errorf("init wrote gsx.toml into an app without gsx:\n%s", read(t, root, "gsx.toml"))
	}
	if s := read(t, root, "intl.toml"); !strings.Contains(s, `locales = ["en"]`) {
		t.Errorf("intl.toml:\n%s", s)
	}
	// A .gsx file of its own makes it a gsx app.
	if err := os.WriteFile(filepath.Join(root, "page.gsx"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !setup.UsesGsx(root, "") {
		t.Error("an app with a .gsx file isn't a gsx app")
	}
}

// TestInitExisting: an app's own gsx.toml keeps everything it has; intl's lines go where TOML needs them.
func TestInitExisting(t *testing.T) {
	gsx := "class_merger = \"example.com/shop/twcfg.Merge\"\nfilter_packages = [\"example.com/shop/filters\"]\n\n[renderers] # the app's own\r\n\"example.com/shop/db.Text\" = \"example.com/shop/db.TextOf\"\n\n[dev]\nno_web = true\n"
	root := app(t, map[string]string{"gsx.toml": gsx, ".gitignore": "/build/\nintl_gen.go", "mise.toml": "[tools]\ngo = \"1.27.3\"\n\n[env]\nX = \"1\"\n"})
	o := setup.Options{Locales: []string{"en"}, Catalogs: "msg"}
	for range 2 { // and again: nothing changes
		if err := setup.Init(root, o, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	var g gsxConfig
	if _, err := toml.Decode(read(t, root, "gsx.toml"), &g); err != nil {
		t.Fatalf("%v:\n%s", err, read(t, root, "gsx.toml"))
	}
	if g.ClassMerger == "" || len(g.FilterPackages) != 2 || len(g.Renderers) != 4 || g.Dev["no_web"] != true {
		t.Errorf("gsx.toml: %+v\n%s", g, read(t, root, "gsx.toml"))
	}
	if s := read(t, root, ".gitignore"); strings.Count(s, "intl_gen.go") != 1 || !strings.Contains(s, "intl-messages.mjs") {
		t.Errorf(".gitignore:\n%s", s)
	}
	var m struct {
		Tools map[string]string `toml:"tools"`
		Env   map[string]string `toml:"env"`
	}
	if _, err := toml.Decode(read(t, root, "mise.toml"), &m); err != nil || m.Tools["go"] == "" || m.Tools["workerd"] != "" || m.Env["X"] != "1" {
		t.Errorf("mise.toml: %v %+v\n%s", err, m, read(t, root, "mise.toml"))
	}

	gsx = "url_presets = [\"htmx\"]\n"
	root = app(t, map[string]string{"gsx.toml": gsx})
	if err := setup.Init(root, o, io.Discard); err != nil {
		t.Fatal(err)
	}
	g = gsxConfig{}
	if _, err := toml.Decode(read(t, root, "gsx.toml"), &g); err != nil || len(g.FilterPackages) != 1 || len(g.Renderers) != 3 {
		t.Errorf("gsx.toml without tables: %v %+v\n%s", err, g, read(t, root, "gsx.toml"))
	}
}

// TestInitAgain: init run again after gsxui init writes gsxui into intl.toml, once, beside catalogs.
func TestInitAgain(t *testing.T) {
	root := app(t, map[string]string{})
	if err := setup.Init(root, setup.Options{Locales: []string{"en", "de"}, Catalogs: "locales"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(t, root, "intl.toml"), "gsxui =") {
		t.Fatal("intl.toml names gsxui before there is one")
	}
	write(t, root, "gsxui.json", `{"ui": "components"}`)
	ui := setup.DetectGsxui(root)
	if ui != "components" {
		t.Fatalf("DetectGsxui = %q, want components", ui)
	}
	for range 2 {
		if err := setup.Init(root, setup.Options{Locales: []string{"en", "de"}, Catalogs: "locales", Gsxui: ui}, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	s := read(t, root, "intl.toml")
	if strings.Count(s, `gsxui = "components"`) != 1 || !strings.Contains(s, "catalogs = \"locales\"\n# Where gsxui add vendors") {
		t.Errorf("intl.toml after init again:\n%s", s)
	}
}

// TestTaskIncludes: an app that lists its task files in mise.toml gets intl's added to the list, once; an app that
// doesn't is left alone (mise finds .mise/tasks itself).
func TestTaskIncludes(t *testing.T) {
	root := app(t, map[string]string{"mise.toml": "[tools]\ngo = \"1.27\"\n\n[task_config]\nincludes = [\"tasks/app.toml\"]\n"})
	for range 2 {
		if err := setup.TaskIncludes(root, ".mise/tasks/mise-intl.toml", io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	var m struct {
		TaskConfig struct{ Includes []string } `toml:"task_config"`
	}
	if _, err := toml.Decode(read(t, root, "mise.toml"), &m); err != nil {
		t.Fatal(err)
	}
	if want := []string{".mise/tasks/mise-intl.toml", "tasks/app.toml"}; !slices.Equal(m.TaskConfig.Includes, want) {
		t.Errorf("includes = %q, want %q", m.TaskConfig.Includes, want)
	}

	// intl's tasks already listed from another file (this repo lists intl/mise/tasks.toml): no second copy.
	listed := app(t, map[string]string{"mise.toml": "[task_config]\nincludes = [\"intl/mise/tasks.toml\"]\n",
		"intl/mise/tasks.toml": "[\"intl:check\"]\nrun = \"charter intl check\"\n"})
	if err := setup.TaskIncludes(listed, ".mise/tasks/mise-intl.toml", io.Discard); err != nil {
		t.Fatal(err)
	}
	if s := read(t, listed, "mise.toml"); strings.Contains(s, ".mise/tasks") {
		t.Errorf("intl's tasks listed twice:\n%s", s)
	}

	plain := app(t, map[string]string{"mise.toml": "[tools]\ngo = \"1.27\"\n"})
	if err := setup.TaskIncludes(plain, ".mise/tasks/mise-intl.toml", io.Discard); err != nil {
		t.Fatal(err)
	}
	if s := read(t, plain, "mise.toml"); s != "[tools]\ngo = \"1.27\"\n" {
		t.Errorf("mise.toml without includes changed:\n%s", s)
	}
}

func write(t *testing.T, root, name, s string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}
