// Command mise-intl is an app's languages, from its intl.toml (found up from the working directory). A repo gets it as
// mise-cloudflare's repos get mise-cloudflare: a binary on mise-intl's GitHub releases (goreleaser, .goreleaser.yaml),
// which mise installs at the release the repo's .mise/config.toml pins (github:joeblew999/mise-intl). The app imports
// intl's packages from the same release's Go module. Each command is a task too, named for it
// (.mise/tasks/mise-intl.toml, which mise-intl init writes): mise run intl:<command> runs mise-intl <command>.
//
// Each command is a file of its own, holding the function named for it (gen.go holds gen), so misecheck holds every
// task of the template to the Go it runs, both ways. mise-intl show-help lists them.
package intl

import (
	"fmt"
	"os"
	"strings"

	"github.com/joeblew999/charter/intl/config"
)

const help = `mise-intl: the app's languages, through the runtime's own Intl. Each command is a task too, with mise,
named for it: mise run intl:<command> runs mise-intl <command>.

The steps, in order (mise-intl, or mise run intl, says which is next):
  mise-intl init            intl's files: intl.toml, the catalogs, the tasks, the skill, the docs, the workflow
                               (--locales en,de --catalogs locales --gsxui ui)
  mise-intl check           every problem, for CI (--missing warn; or only the locales named)
  mise-intl show            what's done, and what to run next (mise-intl, too; --json)
  mise-intl gen             the catalogs compiled into the app's messages package
  mise-intl status          per language: translated, inherited, stale, missing
  mise-intl add-locale      a language added: de, pt-BR
  mise-intl todo            what's left to translate (--json, for agents)
  mise-intl translate       Claude Code translates what's left, a worker per language
  mise-intl export          XLIFF for translators (--all, --out dir)
  mise-intl import          translators' XLIFF read back
  mise-intl confirm         stale translations that are still right, kept (<locale>[:<key>]…)
  mise-intl prune           keys the source no longer has, removed

And:
  mise-intl verify          what the repo's GitHub workflow runs: its setup and test tasks, then check
  mise-intl template        the template's little app, in a repo with no app yet: it never writes over a file
  mise-intl add             intl's screens, in a gsx app (--diff, --overwrite): language-list, language-offer, preferences
  mise-intl assets          the JavaScript a Worker and a page import (--worker dir, --browser dir)
  mise-intl daemon          workerd serving Intl outside a Worker, in the foreground: what pitchfork.toml's intl daemon runs
  mise-intl locales         the shipped locales, comma-separated
  mise-intl catalogs        the catalogs' Go package name
  mise-intl markup          the app's own markup files, for gsx's checks
  mise-intl show-version    this build's version
  mise-intl show-help       this
`

// Main runs charter intl with args, the arguments after "intl".
func Main(args []string) {
	command := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}
	switch command {
	case "":
		status(args)
	case "check":
		check(args)
	case "gen":
		gen(args)
	case "status":
		status(args)
	case "add-locale":
		addLocale(args)
	case "todo":
		todo(args)
	case "translate":
		translate(args)
	case "export":
		export(args)
	case "import":
		importXLIFF(args)
	case "confirm":
		confirm(args)
	case "prune":
		prune(args)
	case "add":
		add(args)
	case "assets":
		assets(args)
	case "daemon":
		daemon(args)
	case "locales":
		locales(args)
	case "catalogs":
		catalogs(args)
	case "markup":
		markup(args)
	case "show-version":
		showVersion()
	case "show-help":
		showHelp()
	default:
		fmt.Fprint(os.Stderr, help)
		os.Exit(2)
	}
}

// app is the app's intl.toml, found up from the working directory.
func app() config.Config {
	c, err := config.Find(".")
	if err != nil {
		fail(fmt.Errorf("%w: run mise-intl init where the app's go.mod is", err))
	}
	return c
}
