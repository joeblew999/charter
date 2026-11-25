// Package seo is charter seo: how a Go web app answers search engines, from its seo.toml (found up from the working
// directory): the live site audited as Googlebot reads it, Lighthouse on Google's servers, the README's badges, and
// Google Search Console. Each command is a file of its own, holding the method named for it (audit.go holds audit).
// The app imports seo's packages from this same module (github.com/joeblew999/charter/site/seo, gsxseo).
//
// Exit codes: 0 ok, 1 problems (or no access), 2 usage. --json prints one JSON document on stdout instead of text (the
// same data, for agents, CI and jq) and never opens a browser.
package seo

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// command is one command of charter seo: its name, what it does and when to run it, each one short line.
type command struct{ name, does, when string }

// commands are every command, in the order a repo meets them.
var commands = []command{
	{"audit", "the site as Googlebot reads it, robots.txt, the sitemap and every page in it: whatever would keep a page out of the index or look wrong in a result (--site: another server, a local build or a preview)", "after every production deploy, and on a local build before it"},
	{"lighthouse", "Lighthouse on the live site on Google's servers: performance, accessibility, best practices and SEO out of 100 for every page or the one named, what fails and on which elements, real users' Core Web Vitals; fails under seo.toml's min (--desktop: the desktop page)", "after a production deploy, and to fix a score"},
	{"badges", "Lighthouse on every page and the audit, then the README's badges and its scores table", "after a production release, then commit them"},
	{"setup", "the Google Cloud project, its Search Console and PageSpeed Insights APIs, and a service account key (asks first; needs gcloud auth login)", "once per person, before the commands that ask Google"},
	{"sites", "the Search Console properties the service account can use; fails without Full access to seo.toml's", "after setup, and after adding the account to a property"},
	{"submit", "the sitemap submitted to Search Console, after a passing audit (asks first)", "once the site is live, and after its pages change"},
	{"status", "the sitemap's status in Search Console, and Google's index status of every URL in it", "to see what Google has of the site"},
	{"todo", "Google's verdict for every sitemap URL: indexed, waiting, duplicate or a problem; fails on a problem (--open: each problem's Search Console page)", "days after submitting, and whenever Search Console reports a problem"},
	{"inspect", "one URL, the home page or the one named: Google's verdict, last crawl and canonical, and its Search Console page (--open opens it)", "to look into one page"},
	{"help", "every command and its flags", "to list every command"},
}

// help is every command, from commands, and the flags.
var help = func() string {
	var b strings.Builder
	b.WriteString("charter seo: how the app answers search engines, from its seo.toml.\n\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "  charter seo %-12s %s\n", c.name, c.does)
	}
	b.WriteString(`
--json, one JSON document on stdout instead of text: audit, lighthouse, sites, status, todo, inspect. --site <url>:
another server for audit. lighthouse and inspect take a page, a path (/de/) or a full URL. The commands that ask Google
read GOOGLE_SEARCH_CONSOLE_KEY and PAGESPEED_API_KEY from the environment.
`)
	return b.String()
}()

// Main runs charter seo with args, the arguments after "seo".
func Main(args []string) {
	command := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}
	dir, err := os.Getwd()
	must(err)
	r := repo{dir: dir}
	switch command {
	case "audit":
		r.audit(args)
	case "lighthouse":
		r.lighthouse(args)
	case "badges":
		r.badges(args)
	case "setup":
		r.setup(args)
	case "sites":
		r.sites(args)
	case "submit":
		r.submit(args)
	case "status":
		r.status(args)
	case "todo":
		r.todo(args)
	case "inspect":
		r.inspect(args)
	case "show-version":
		showVersion(args)
	case "show-help", "help":
		showHelp()
	default:
		fmt.Fprint(os.Stderr, help)
		os.Exit(2)
	}
}

// repo is the repo in the folder charter seo runs in.
type repo struct{ dir string }

// flags is a command's flag set: one it doesn't take is refused (exit 2), with every command's.
func flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet("charter seo "+name, flag.ExitOnError)
	f.Usage = func() { fmt.Fprint(os.Stderr, help) }
	return f
}

// parse parses a command's flags before or after its arguments (charter seo inspect /de/ --json is charter seo inspect
// --json /de/), and returns the arguments: more than most is a usage error.
func parse(f *flag.FlagSet, args []string, most int) []string {
	var out []string
	for {
		f.Parse(args)
		if f.NArg() == 0 {
			break
		}
		out, args = append(out, f.Arg(0)), f.Args()[1:]
	}
	if len(out) > most {
		f.Usage()
		os.Exit(2)
	}
	return out
}
