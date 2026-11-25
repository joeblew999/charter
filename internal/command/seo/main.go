// Command mise-seo is how a Go web app's repo answers search engines, from its seo.toml (found up from the working
// directory): the live site audited as Googlebot reads it, Lighthouse on Google's servers, the README's badges, and
// Google Search Console. A repo gets it as it gets mise-intl: a binary on mise-seo's GitHub releases (goreleaser,
// .goreleaser.yaml), which mise installs at the release the repo's .mise/config.toml pins (github:joeblew999/mise-seo).
// The app imports mise-seo's packages from the same release's Go module. Each command is a task too
// (.mise/tasks/mise-seo.toml, which mise-seo init writes): mise run seo:<command> runs mise-seo <command>, and mise run
// seo says which to run next.
//
// Each command is a file of its own, holding the method named for it (audit.go holds audit), so misecheck holds every
// task of the template to the Go it runs, both ways. mise-seo show-help lists them.
//
// Exit codes are the contract between tasks: 0 ok, 1 problems (or no access), 2 usage. --json prints one JSON document
// on stdout instead of text (the same data, for agents, CI and jq) and never opens a browser.
package seo

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// command is one command of mise-seo as its task runs it: the task, what the command does and when to run it, each
// one short line. This table is the one source of those lines: show-help prints them, and TestCommands holds the
// template's tasks file (each task's description is what it does), its skill and its docs page (each task's row is
// both) to them, word for word.
type command struct{ task, does, when string }

// commands are every command, as the template's tasks file has them, in its order: the steps in the order a repo
// meets them, then the rest.
var commands = []command{
	{"seo", "what's done for the repo, and what to run next", "first, and whenever you're unsure"},
	{"seo:init", "mise-seo's files (its tasks, the workflow, the skill, the docs, the hook), seo.toml when the repo has none or no site (--site, the live site), and go.mod requiring mise-seo's packages at this release", "once, then after upgrading mise-seo"},
	{"seo:check", "the repo's rules: mise-seo's files as this version writes them, its pin, seo.toml with a site, the packages in go.mod at the binary's release", "after a change to seo.toml or mise-seo's files; when it fails, run what it names"},
	{"seo:show", "the same as mise run seo", "to see where the repo is"},
	{"seo:audit", "the site as Googlebot reads it, robots.txt, the sitemap and every page in it: whatever would keep a page out of the index or look wrong in a result (--site: another server, a local build or a preview)", "after every production deploy, and on a local build before it"},
	{"seo:lighthouse", "Lighthouse on the live site on Google's servers: performance, accessibility, best practices and SEO out of 100 for every page or the one named, what fails and on which elements, real users' Core Web Vitals; fails under seo.toml's min (--desktop: the desktop page)", "after a production deploy, and to fix a score"},
	{"seo:badges", "Lighthouse on every page and the audit, then the README's badges and its scores table", "after a production release, then commit them"},
	{"seo:setup", "the Google Cloud project, its Search Console and PageSpeed Insights APIs, and a service account key in fnox (asks first; needs gcloud auth login)", "once per person, before the tasks that ask Google"},
	{"seo:sites", "the Search Console properties the service account can use; fails without Full access to seo.toml's", "after setup, and after adding the account to a property"},
	{"seo:submit", "the sitemap submitted to Search Console, after a passing audit (asks first)", "once the site is live, and after its pages change"},
	{"seo:status", "the sitemap's status in Search Console, and Google's index status of every URL in it", "to see what Google has of the site"},
	{"seo:todo", "Google's verdict for every sitemap URL: indexed, waiting, duplicate or a problem; fails on a problem (--open: each problem's Search Console page)", "days after submitting, and whenever Search Console reports a problem"},
	{"seo:inspect", "one URL, the home page or the one named: Google's verdict, last crawl and canonical, and its Search Console page (--open opens it)", "to look into one page"},
	{"seo:verify", "what the repo's GitHub workflow runs: its setup and test tasks, then check", "before every push"},
	{"seo:template", "the template's little app, its pages answering search engines as the audit wants; it never writes over a file", "only in a repo with no Go program yet"},
	{"seo:show-version", "its version", "to say which mise-seo this is"},
	{"seo:show-help", "every command and its flags", "to list every command"},
}

// help is show-help's text: every command, from commands, and the flags.
var help = func() string {
	var b strings.Builder
	b.WriteString("mise-seo: how the app answers search engines, from its seo.toml. Each command is a task, mise run seo:<command>,\nin the order a repo meets them:\n\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "  %-32s %s\n", "mise run "+c.task, c.does)
	}
	b.WriteString(`
--json, one JSON document on stdout instead of text: mise-seo, show, check, audit, lighthouse, sites, status, todo,
inspect. --site <url>: init's live site for seo.toml, or another server for audit. lighthouse and inspect take a page,
a path (/de/) or a full URL. The tasks that ask Google read GOOGLE_SEARCH_CONSOLE_KEY and PAGESPEED_API_KEY from the
environment, else from fnox. Start with mise run seo: it says what to run next.
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
	case "show-help":
		showHelp()
	default:
		fmt.Fprint(os.Stderr, help)
		os.Exit(2)
	}
}

// repo is the repo in the folder mise-seo runs in.
type repo struct{ dir string }

// flags is a command's flag set: one it doesn't take is refused (exit 2), with every command's.
func flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet("mise-seo "+name, flag.ExitOnError)
	f.Usage = func() { fmt.Fprint(os.Stderr, help) }
	return f
}

// parse parses a command's flags before or after its arguments (mise-seo inspect /de/ --json is mise-seo inspect
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
