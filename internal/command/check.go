package command

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/dbcode"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"path/filepath"
	"strings"
)

// Check runs charter check: everything that can be said without running the app. It starts nothing, so it is safe
// beside a running charter dev. --fix writes what it can.
//
//	generate   the messages, the components' Go and the browser's files, which everything after reads
//	fmt        charter fmt --check (with --fix: charter fmt)
//	lint       charter lint (with --fix: go mod tidy)
//	contract   the committed specs, Fern's settings and the queries' Go are what the contract gives (with --fix: they
//	           are written), and each rate limit the contract declares is in wrangler.jsonc
//	languages  every language complete, plural forms, unused messages, gsxui's own text, right-to-left set up
func Check(args []string) {
	fs := flag.NewFlagSet("charter check", flag.ExitOnError)
	fix := fs.Bool("fix", false, "write what can be written: formatting, go.mod, the specs and the queries' Go")
	fs.Parse(args)
	p := load()
	check(skills(p, false)) // the agent skills of what the app is built with, when go.sum has moved
	t := newTimes("check")
	if p.Spec != "" && p.DB != "" && *fix {
		t.part("queries", func() error { return dbcode.Write(p.Root, p.DB, false) })
	}
	t.part("generate", func() error { return generate(p) })
	t.part("fmt", func() error { return format(p, *fix) })
	if *fix {
		t.part("tidy", func() error { return run.Run(p.Root, nil, "go", "mod", "tidy") })
	}
	t.part("lint", func() error { return lint(p) })
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
