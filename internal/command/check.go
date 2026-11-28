package command

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/accounts"
	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/dbcode"
	"github.com/joeblew999/charter/internal/docsgen"
	"github.com/joeblew999/charter/internal/envcode"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/scopecode"
	"github.com/joeblew999/charter/internal/themes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Check runs charter check: everything that can be said without running the app. It starts nothing, so it is safe
// beside a running charter dev. --fix writes what it can.
//
//	tools      Go, Node and fnox are the versions mise.toml pins, and it pins at least what charter was tested with
//	           (with --fix: the pins are raised and installed)
//	themes     for an app with the theme screen: gsxui's theme editor is in the app as gsxui has it now, its markup
//	           and catalogue and its scripts (with --fix: written)
//	generate   the messages, the components' Go and the browser's files, which everything after reads
//	fmt        charter fmt --check (with --fix: charter fmt)
//	lint       charter lint (with --fix: go mod tidy)
//	statements no statement on the database is written in Go: each is in db/queries.sql, checked against
//	           db/schema.sql and written as a function. And the schema still has the sign-in screen's tables as
//	           charter reads and writes them
//	bindings   no binding, variable or secret is named as text: each is its value in env, written from wrangler.jsonc
//	scopes     a scope is declared once, as a package's variable, and named everywhere else
//	contract   the committed specs, Fern's settings and the queries' Go are what the contract gives (with --fix: they
//	           are written), and each rate limit the contract declares is in wrangler.jsonc
//	languages  every language complete, plural forms, unused messages, gsxui's own text, right-to-left set up
func Check(args []string) {
	fs := flag.NewFlagSet("charter check", flag.ExitOnError)
	fix := fs.Bool("fix", false, "write what can be written: formatting, go.mod, the specs, the migration for db/schema.sql and the queries' Go")
	drop := fs.Bool("drop", false, "with --fix: write a migration that drops a column or a table with its rows, which is otherwise refused")
	asJSONFlag := fs.Bool("json", false, "one JSON document on stdout: whether it passed, and each part with what it said (for an agent or a script)")
	fs.Parse(args)
	if *asJSONFlag {
		asJSON("check", slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--json" || a == "-json" }))
		return
	}
	p := load()
	t := newTimes("check")
	// The tools first: what everything after runs on.
	t.part("tools", func() error { return toolchain(p, *fix) })
	if p.Spec != "" && p.DB != "" && *fix {
		t.part("queries", func() error { return dbcode.Write(p.Root, p.DB, false, *drop) })
	}
	if exists(filepath.Join(p.Root, "docs")) {
		// A sample of the docs that is taken from a file (its fence says file=…) is what the file says.
		t.part("docs", func() error {
			differ, err := docsgen.Samples(p.Root, *fix)
			if err != nil || len(differ) == 0 || *fix {
				return err
			}
			return fmt.Errorf("these samples of the docs are not what their files say (charter check --fix writes them again):\n  %s", strings.Join(differ, "\n  "))
		})
	}
	// Before the screens, which take their components from it.
	t.part("styles", func() error { return themes.WriteStyles(p.Root, !*fix) })
	if hasIntl(p) && exists(filepath.Join(p.Root, "pages", "screens")) {
		// charter's screens the app has and has not changed follow charter, as its library code does. Before anything
		// is generated from them: a screen written for an older charter may not compile against this one, and saying
		// it is behind is what helps, not the compiler's error.
		t.part("screens", func() error {
			behind, err := intl.ScreensBehind()
			if err != nil || len(behind) == 0 {
				return err
			}
			if !*fix {
				return fmt.Errorf("charter writes these screens differently now, and the app has not changed its copies: %s (charter check --fix brings them up to date)", strings.Join(behind, ", "))
			}
			say("screens brought up to date: %s", strings.Join(behind, ", "))
			return intl.UpdateScreens(behind)
		})
	}
	if exists(filepath.Join(p.Root, "pages", "screens", "theme.gsx")) {
		t.part("themes", func() error { return themesWritten(p, *fix) })
	}
	// One document around every page: gsxui's own site's shell, as charter's layout screen has it.
	t.part("layout", func() error {
		own, err := themes.OwnShells(p.Root)
		if err != nil || len(own) == 0 {
			return err
		}
		return fmt.Errorf("these write out what every page has from one place (pages/screens/layout.gsx), so each would look its own way:\n  %s", strings.Join(own, "\n  "))
	})
	t.part("generate", func() error { return generate(p) })
	t.part("fmt", func() error { return format(p, *fix) })
	if *fix {
		t.part("tidy", func() error { return run.Run(p.Root, nil, "go", "mod", "tidy") })
	}
	if *fix && exists(filepath.Join(p.Root, "gsxui.json")) {
		t.part("components", func() error { return componentsRecorded(p) })
	}
	if *fix && exists(filepath.Join(p.Root, "pages")) {
		if b, _ := os.ReadFile(filepath.Join(p.Root, "go.mod")); !strings.Contains(string(b), linter) {
			t.part("linter", func() error { return run.Run(p.Root, nil, "go", "get", "-tool", linter) })
		}
	}
	t.part("lint", func() error { return lint(p) })
	if exists(filepath.Join(p.Root, "db", "schema.sql")) {
		t.part("statements", func() error { return statementsTyped(p) })
	}
	t.part("bindings", func() error { return bindingsTyped(p) })
	t.part("scopes", func() error { return scopesDeclared(p) })
	if p.Spec != "" {
		t.part("contract", func() error {
			if err := spec(p, !*fix); err != nil {
				return err
			}
			return limitsDeclared(p)
		})
	}
	if hasIntl(p) && *fix {
		// The languages wired into gsx where they are not (gsx.toml, gsxui's text), and gsxui's components rewritten
		// for a right-to-left language among them, once.
		t.part("languages wired", func() error {
			did, err := intl.FixRTL(func() error { return run.Run(p.Root, nil, "go", "tool", "gsxui", "migrate", "rtl") })
			if did {
				say("gsxui.json: \"rtl\": true, and the components rewritten for right-to-left languages")
			}
			return err
		})
	}
	if hasIntl(p) {
		t.part("languages", func() error {
			if !intl.Check() {
				return errors.New("the languages have problems (above)")
			}
			return nil
		})
	}
	t.done()
}

// bindingsTyped fails where the app's Go names a binding, a variable or a secret as text: a name that compiles
// whatever it says, and fails on a Worker when nothing of that name is bound, or something of another kind is.
// Each is a value in env (env/env.built.go, written from wrangler.jsonc, .dev.vars.example and fnox.toml by
// generate, before this), of its kind's type.
func bindingsTyped(p *project.Project) error {
	named, err := envcode.Lint(p.Root)
	if err != nil || len(named) == 0 {
		return err
	}
	return fmt.Errorf("these name a binding, a variable or a secret as text, which compiles whatever it says:\n  %s\nuse its value in env, which charter writes from wrangler.jsonc, .dev.vars.example and fnox.toml: env.DB.Open(), env.KV.Open(), env.WRITE_LIMIT.Allow(key), env.APP_NAME.Value(). One that is not there is not declared: declare it (charter add d1:DB, or the file)",
		strings.Join(named, "\n  "))
}

// themesWritten holds an app that has gsxui's theme editor (the theme screen) to having it as gsxui has it now:
// the editor's markup and catalogue, rendered by gsxui's own Go, and its scripts (internal/themes), written when
// the app's preset or its gsxui has changed, and loaded by the page (web/main.js imports what connects them).
// With fix all of it is written; without, what is behind is said.
func themesWritten(p *project.Project, fix bool) error {
	if err := themes.Write(p.Root, !fix); err != nil {
		return err
	}
	if direct, err := themes.Unstyled(p.Root); err != nil {
		return err
	} else if len(direct) > 0 {
		return fmt.Errorf("these take their components from ui itself, which is the app's own style whatever a reader chose in the theme editor:\n  %s\nimport ui %q instead: the same components, each in the reader's style", strings.Join(direct, "\n  "), themes.StylesImport(p.Root))
	}
	entry := filepath.Join(p.Root, "web", "main.js")
	b, err := os.ReadFile(entry)
	if err != nil {
		return fmt.Errorf("gsxui's theme editor is loaded by the page's script, and web/main.js is not there: %w", err)
	}
	const after = `import "./gsxui/index.css";`
	text := string(b)
	for _, load := range []struct{ line, why string }{
		{`import "./gsxui-site/site.css";`, "// How gsxui's own docs look, for the docs' pages: gsxui's stylesheet, which charter check --fix writes from gsxui."},
		{`import "./gsxui-site/charter.js";`, "// gsxui's theme editor (the theme screen), and a page of the app shown inside it as its live preview: charter check\n// --fix writes the folder from gsxui itself."},
	} {
		if strings.Contains(text, load.line) {
			continue
		}
		if !fix {
			return fmt.Errorf("web/main.js does not load what charter writes from gsxui (web/gsxui-site): add %s after %s (charter check --fix does)", load.line, after)
		}
		if !strings.Contains(text, after) {
			return fmt.Errorf("web/main.js has no %s to put %s after: add it where the app's CSS is loaded", after, load.line)
		}
		say("web/main.js: %s", load.line)
		text = strings.Replace(text, after, after+"\n"+load.why+"\n"+load.line, 1)
	}
	if text == string(b) {
		return nil
	}
	return os.WriteFile(entry, []byte(text), 0o644)
}

// scopesDeclared fails where the app's Go makes a scope that it is not declaring: a scope is a value so that no
// word is written where one is needed, and NewScope outside a declaration is the word written by hand again.
func scopesDeclared(p *project.Project) error {
	made, err := scopecode.Lint(p.Root)
	if err != nil || len(made) == 0 {
		return err
	}
	return fmt.Errorf("these make a scope where it is used, from a word that compiles whatever it says:\n  %s\ndeclare each scope once, as a package's variable, and name it: var Write = charter.NewScope(\"write\"), then Needs() []charter.Scope { return []charter.Scope{api.Write} }",
		strings.Join(made, "\n  "))
}

// statementsTyped fails where the app's Go gives the database a statement as text, or reaches it around charter:
// what sqlc has not checked against db/schema.sql is found on a Worker, by whoever runs into it. And it fails an
// app whose schema no longer has the tables of charter's sign-in as charter's own statements need them
// (internal/accounts): the app owns its copy of them, and may add to it.
func statementsTyped(p *project.Project) error {
	written, err := dbcode.HandWritten(p.Root)
	if err != nil {
		return err
	}
	if len(written) > 0 {
		return fmt.Errorf("these give the database a statement written in Go, which nothing checks against db/schema.sql:\n  %s\nwrite it in db/queries.sql (-- name: Something :one, :many, :exec or :execrows) and call the function charter check --fix writes for it in db/queries.gen.go; for several in one transaction, cloud.Batch of their …Stmt functions",
			strings.Join(written, "\n  "))
	}
	if exists(filepath.Join(p.Root, "pages", "screens", "sign-in.gsx")) {
		return dbcode.Fits(p.Root, "the sign-in screen (users, passkeys, tokens, ceremonies)", accounts.Schema, accounts.Queries)
	}
	return nil
}

// limitsDeclared fails when the contract declares a rate limit (x-rate-limit on an operation of the spec) that
// wrangler.jsonc does not bind, or binds with another limit or period: the Worker would count against nothing.
func limitsDeclared(p *project.Project) error {
	b, err := os.ReadFile(filepath.Join(p.Root, p.Spec))
	if err != nil {
		return err
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("%s: %w", p.Spec, err)
	}
	for path, ops := range doc.Paths {
		for method, raw := range ops {
			var op struct {
				Limit *struct {
					Binding string `json:"binding"`
					Limit   int    `json:"limit"`
					Period  int    `json:"period"`
				} `json:"x-rate-limit"`
			}
			if json.Unmarshal(raw, &op) != nil || op.Limit == nil {
				continue
			}
			have, ok := p.RateLimit(op.Limit.Binding)
			want := fmt.Sprintf(`{ "name": %q, "namespace_id": "<a number of your own>", "simple": { "limit": %d, "period": %d } }`, op.Limit.Binding, op.Limit.Limit, op.Limit.Period)
			switch {
			case !ok:
				return fmt.Errorf("%s %s counts against the rate limit %s, which %s does not bind: add to its ratelimits %s", strings.ToUpper(method), path, op.Limit.Binding, project.Wrangler, want)
			case have.Simple.Limit != op.Limit.Limit || have.Simple.Period != op.Limit.Period:
				return fmt.Errorf("the rate limit %s is %d per %d s in the contract and %d per %d s in %s: make them the same", op.Limit.Binding, op.Limit.Limit, op.Limit.Period, have.Simple.Limit, have.Simple.Period, project.Wrangler)
			}
		}
	}
	return nil
}

// componentsRecorded has gsxui.json say again what gsxui wrote, for a component that is exactly gsxui's and whose
// record is not: an app made from another (charter create) has its components under a new module path, and gsxui
// would take them for changed by hand and refuse to restyle them (charter run ui:theme). A component the app did
// change is left as it is, and stays the app's.
func componentsRecorded(p *project.Project) error {
	b, err := os.ReadFile(filepath.Join(p.Root, "gsxui.json"))
	if err != nil {
		return err
	}
	var conf struct {
		UI      string            `json:"ui"`
		Managed map[string]string `json:"managed"`
	}
	if err := json.Unmarshal(b, &conf); err != nil {
		return err
	}
	var stale []string
	for rel, want := range conf.Managed {
		name, isComponent := strings.CutSuffix(strings.TrimPrefix(rel, conf.UI+"/"), ".gsx")
		if !isComponent || strings.Contains(name, "/") {
			continue
		}
		file, err := os.ReadFile(filepath.Join(p.Root, rel))
		if err != nil {
			continue
		}
		if fmt.Sprintf("%x", sha256.Sum256(file)) != want {
			stale = append(stale, name)
		}
	}
	for _, name := range stale {
		// gsxui's own word on whether the file is what it writes.
		diff := exec.Command("go", "tool", "gsxui", "add", "-diff", name)
		diff.Dir = p.Root
		out, err := diff.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "(no changes)") {
			continue
		}
		if err := run.Run(p.Root, nil, "go", "tool", "gsxui", "add", "-overwrite", name); err != nil {
			return err
		}
	}
	return nil
}
