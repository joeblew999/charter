package intl

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/intl/config"
	"github.com/joeblew999/charter/intl/intltest"
)

// check is charter intl check: every problem, for CI: syntax, arguments, plural categories, completeness, unused
// messages, gsxui text, intl.sum, and intl's files as this version writes them (exit 1 if any). With locales, only
// theirs, writing nothing; --missing warn only reports missing and stale translations (a contributor's pull request).
// Without --missing, INTL_MISSING says, else GitHub Actions' pull_request runs only warn: a contributor's pull request
// may leave its translations to the maintainer, who translates after merging. Pushes and releases stay strict.
func check(args []string) {
	c := app()
	f := flag.NewFlagSet("check", flag.ExitOnError)
	missing := f.String("missing", missingDefault(), `missing or stale translations: "fail", or "warn" (a contributor's pull request, which the maintainer translates after merging)`)
	f.Parse(args)
	if *missing != "fail" && *missing != "warn" {
		fail(fmt.Errorf("intl check: --missing %q: fail or warn", *missing))
	}
	if *missing == "warn" && os.Getenv("INTL_MISSING") == "" && os.Getenv("GITHUB_EVENT_NAME") == "pull_request" {
		fmt.Println("· a pull request (GITHUB_EVENT_NAME=pull_request): missing and stale translations only warn; pushes and releases stay strict (INTL_MISSING=fail to fail)")
	}
	if f.NArg() > 0 {
		os.Exit(checkLocales(c, f.Args()))
	}
	os.Exit(checkAll(c, *missing == "warn"))
}

// Check is the whole check for the app here, as charter check runs it among its own parts: it reports whether every
// rule holds, and prints each problem.
func Check() bool { return checkAll(app(), missingDefault() == "warn") == 0 }

// checkLocales checks only the named locales' own catalogs (and what they inherit from): syntax, arguments,
// completeness, staleness and plural categories. It reads and writes nothing else, so a translator working on one
// language runs it while others work on theirs.
func checkLocales(c config.Config, ids []string) int {
	keep := []string{c.Locales[0]}
	for _, id := range ids {
		for from := id; from != "" && !slices.Contains(keep, from); from = c.Inherit[from] {
			if i := slices.IndexFunc(c.Locales, func(l string) bool { return strings.EqualFold(l, from) }); i >= 0 {
				keep = append(keep, c.Locales[i])
			}
		}
	}
	scoped := c
	scoped.Locales = keep
	p, err := catalog.Read(scoped)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	locs := pick(p, ids)
	problems := localeProblems(p, locs)
	startIntl()
	pl, err := intltest.PluralProblems(p)
	must(err)
	for _, s := range pl {
		for _, l := range locs {
			if strings.HasPrefix(s, l.File+" ") {
				problems = append(problems, s)
			}
		}
	}
	for _, s := range problems {
		fmt.Println("✗", s)
	}
	if len(problems) > 0 {
		return 1
	}
	var names []string
	for _, l := range locs {
		names = append(names, l.ID)
	}
	fmt.Println("✓", strings.Join(names, ", "))
	return 0
}

// localeProblems are what is left in locs: missing and stale translations, and keys the source no longer has.
func localeProblems(p *catalog.Project, locs []*catalog.Locale) []string {
	var out []string
	for _, l := range locs {
		if n := l.Count(catalog.Missing) + l.Count(catalog.Stale); n > 0 {
			out = append(out, l.Summary(len(p.Messages))+": charter intl translate "+l.ID+" (or todo)")
		}
		if len(l.Obsolete) > 0 {
			out = append(out, fmt.Sprintf("%s has %s, which the source no longer does: charter intl prune", l.File, strings.Join(l.Obsolete, ", ")))
		}
	}
	return out
}

// checkAll prints every problem and returns the exit status.
// checkAll is every problem; with warnMissing, missing and stale translations are only reported.
func checkAll(c config.Config, warnMissing bool) int {
	p, err := catalog.Read(c)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	var problems, warnings []string
	add := func(s ...string) { problems = append(problems, s...) }
	if warnMissing {
		for _, l := range p.Locales {
			if n := l.Count(catalog.Missing) + l.Count(catalog.Stale); n > 0 {
				warnings = append(warnings, l.Summary(len(p.Messages))+": the maintainer translates it after merging")
			}
		}
		for _, s := range localeProblems(p, p.Locales) {
			if !strings.Contains(s, ": charter intl translate ") {
				add(s)
			}
		}
	} else {
		add(localeProblems(p, p.Locales)...)
	}
	if !p.SumCurrent() {
		add(p.SumFile() + " is not up to date: charter intl gen, and commit it")
	}
	if !p.BadgeCurrent() {
		add(p.BadgeFile() + " is not up to date: charter intl gen, and commit it")
	}
	gs, err := p.GsxuiProblems()
	must(err)
	add(gs...)
	unused, err := p.Unused()
	must(err)
	for _, k := range unused {
		add(fmt.Sprintf("%s: no Go or gsx file uses it (%s.%s): remove it from every catalog", k, filepath.Base(c.CatalogDir()), catalog.GoName(k)))
	}
	startIntl()
	pl, err := intltest.PluralProblems(p)
	must(err)
	add(pl...)
	for child, from := range c.Inherit { // inheriting text only makes sense in the same script
		script := func(id string) string {
			s, _ := intl.NewLocale(id).Intl(intl.Call{API: "Locale", Method: "maximize"}, "", "")
			if parts := strings.Split(s, "-"); len(parts) > 1 {
				return parts[1]
			}
			return ""
		}
		if a, b := script(child), script(from); a != b {
			add(fmt.Sprintf("%s: [inherit] %s = %q, but Intl writes %s in %s and %s in %s: give it its own translations", config.File, child, from, child, a, from, b))
		}
	}
	for _, id := range c.Locales { // as Intl writes it: fr-CA, zh-Hant
		canon, ok := intl.NewLocale("en").Intl(intl.Call{API: "Intl", Method: "getCanonicalLocales", Args: []string{intl.JSON(id)}}, "", "")
		if want := `["` + id + `"]`; !ok || canon != want {
			add(fmt.Sprintf("%s: %q isn't a locale id as Intl writes it (%s)", config.File, id, canon))
		}
	}
	// A right-to-left language needs gsxui's components vendored with logical direction classes (gsxui's RTL guide,
	// https://ui.gsxhq.dev/docs/rtl): "rtl": true in gsxui.json, which gsxui init --rtl sets.
	if c.Gsxui != "" {
		var rtl []string
		for _, id := range c.Locales {
			if intl.NewLocale(id).Dir() == "rtl" {
				rtl = append(rtl, id)
			}
		}
		var gsxui struct {
			RTL bool `json:"rtl"`
		}
		b, _ := os.ReadFile(filepath.Join(c.Root, "gsxui.json"))
		json.Unmarshal(b, &gsxui)
		if len(rtl) > 0 && !gsxui.RTL {
			add(fmt.Sprintf("%s is written right to left, and gsxui.json has no \"rtl\": true: gsxui init --rtl sets it (in a project it has set up already: add the line, then gsxui migrate rtl), and the vendored components get logical direction classes (https://ui.gsxhq.dev/docs/rtl)", strings.Join(rtl, ", ")))
		}
	}
	for _, s := range warnings {
		fmt.Println("·", s)
	}
	for _, s := range problems {
		fmt.Println("✗", s)
	}
	if len(problems) > 0 {
		return 1
	}
	fmt.Printf("✓ %d messages in %d locales\n", len(p.Messages), len(p.Locales))
	return 0
}

// missingDefault is check's --missing when it isn't given: INTL_MISSING, else "warn" on a GitHub Actions pull_request
// run, else "fail".
func missingDefault() string {
	if v := os.Getenv("INTL_MISSING"); v != "" {
		return v
	}
	if os.Getenv("GITHUB_EVENT_NAME") == "pull_request" {
		return "warn"
	}
	return "fail"
}
