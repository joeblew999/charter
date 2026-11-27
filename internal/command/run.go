package command

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/command/seo"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// task is a built-in task of charter run.
type task struct {
	name, about string
	run         func(p *project.Project, args []string)
}

// tasks are what charter run has for every app: what is no verb of its own. An app adds its own in charter.toml's
// [run.tasks].
var tasks = []task{
	// The languages (intl).
	{"intl:status", "per language: translated, inherited, stale, missing; with languages named (de fr), their catalogs checked", func(_ *project.Project, a []string) {
		if len(a) > 0 {
			intl.Main(append([]string{"check"}, a...))
		}
		intl.Main([]string{"status"})
	}},
	{"intl:translate", "Claude Code translates what is missing or stale, a worker per language (charter run intl:translate de)", func(_ *project.Project, a []string) { intl.Main(append([]string{"translate"}, a...)) }},
	{"intl:todo", "what is left to translate, with what a translator needs (--json)", func(_ *project.Project, a []string) { intl.Main(append([]string{"todo"}, a...)) }},
	{"intl:confirm", "stale translations that are still right, kept (de, or de:home.title)", func(_ *project.Project, a []string) { intl.Main(append([]string{"confirm"}, a...)) }},
	{"intl:export", "XLIFF for translators (--all, --out dir)", func(_ *project.Project, a []string) { intl.Main(append([]string{"export"}, a...)) }},
	{"intl:import", "translators' XLIFF read back", func(_ *project.Project, a []string) { intl.Main(append([]string{"import"}, a...)) }},
	{"intl:prune", "messages the source no longer has, removed from every catalog", func(_ *project.Project, a []string) { intl.Main(append([]string{"prune"}, a...)) }},
	// Search engines (seo).
	{"seo:audit", "the live site as a search engine reads it (--site: another address)", func(_ *project.Project, a []string) { seo.Main(append([]string{"audit"}, a...)) }},
	{"seo:lighthouse", "Lighthouse on the live site, on Google's servers", func(_ *project.Project, a []string) { seo.Main(append([]string{"lighthouse"}, a...)) }},
	{"seo:badges", "Lighthouse and the audit, then the README's badges and scores table", func(_ *project.Project, a []string) { seo.Main(append([]string{"badges"}, a...)) }},
	{"seo:search", "Google Search Console: setup, sites, submit, status, todo, inspect (charter run seo:search status)", func(_ *project.Project, a []string) {
		if len(a) == 0 {
			a = []string{"status"}
		}
		seo.Main(a)
	}},
	// The database (D1, through wrangler). The local one by default; --deployed is the deployed one.
	{"db:status", "which migrations of db/migrations have run", db("migrations", "list")},
	{"db:migrate", "the migrations that have not run, applied (charter dev and charter deploy do this themselves; the migrations are written from db/schema.sql by charter check --fix)", db("migrations", "apply")},
	{"db:execute", "one statement: charter run db:execute \"SELECT count(*) FROM notes\"", dbExecute},
	{"db:seed", "the local database made again and filled by the app's seed: seed.go (Go, the app's own types and functions) or db/seed.sql. --deployed: what the deployed one has, and with the Worker's name (--deployed my-worker) every row deleted and the seed's put there", dbSeed},
	{"db:reset", "the local database made again from db/migrations, and seeded when the app has a seed", dbReset},
	{"db:export", "the database as SQL: charter run db:export backup.sql", dbExport},
	// The components (gsxui).
	{"ui:list", "the components charter add ui:<name> accepts", func(p *project.Project, a []string) {
		check(run.Run(p.Root, nil, "go", append([]string{"tool", "gsxui", "list"}, a...)...))
	}},
	{"ui:theme", "the app's style and theme (gsxui's presets): charter run ui:theme shows it; charter run ui:theme <preset> switches to a file, a share code from https://ui.gsxhq.dev/theme, or a URL (--only theme|style)", uiTheme},
	// The Worker.
	{"logs", "the deployed Worker's logs: live until Ctrl-C (wrangler tail), or with --since 1h what Cloudflare kept, --errors for only what went wrong, --find <text> for a request's reference (an environment first: charter run logs staging)", logs},
	{"people", "the app's own people (the sign-in screen): who signed up, and what each is (--deployed for the deployed Worker)", people},
	{"people:invite", "a link that makes whoever signs up through it, once, within the hour, a role: charter run people:invite admin (--deployed for the deployed Worker). How an app gets its first admin", peopleInvite},
	{"mail", "the domains that send email in the account (Cloudflare Email Sending)", mail},
	{"mail:enable", "onboard a domain to send email from, to anyone: charter run mail:enable mail.example.com (a subdomain keeps its records apart from the domain's own mail; needs Workers Paid)", mailEnable},
	{"secrets", "the secrets the app names (.dev.vars.example), and which fnox has for the deployed Worker, from the app's fnox.toml (an environment first: charter run secrets staging, its profile)", secrets},
	{"secrets:put", "a secret declared in the app's fnox.toml, kept by fnox's provider and put on the deployed Worker: charter run secrets:put ADMIN_TOKEN --random (a new random value), or the value fnox has, or one piped in (an environment first: charter run secrets:put staging ADMIN_TOKEN --random)", secretsPut},
	{"health", "how the deployed Worker is doing, from Cloudflare's count of how each request ended: the ones it never answered (over its memory, over its CPU time, an exception) are found here or not at all. charter run health --since 6h; an environment first", healthTask},
	{"resources", "the Worker's database, KV namespace, bucket and queue: what wrangler.jsonc names and what the deployed Worker is bound to (an environment first: charter run resources staging)", resources},
	{"delete", "the deployed Worker taken away, with its secrets: alone it says what there is; with the Worker's name it deletes (charter run delete my-worker). Its data stays, and charter deploy brings the app back; --data deletes the database, bucket, queue and KV namespace too", deleteWorker},
	{"load", "the Worker's pages asked many at once for a few seconds, with readers' streams held open: how many it answers a second, how fast, and whether Cloudflare ended any. Before a release, never part of a deploy: charter run load --deployed (-at 20 -for 10s -streams 10; an environment first)", loadTask},
	{"cold", "what a reader waits where no instance of the deployed Worker is running yet: one page asked from many places in the world, twice, through Globalping, with the Worker saying which answers were a first hit. After a change to what the Worker loads or does as it starts, never part of a deploy: charter run cold (-from 30 -path /; an environment first)", coldTask},
	{"bench", "how long the Worker takes to answer, on wrangler dev: charter run bench /  /api/hello  (-n requests each)", bench},
}

// Run runs charter run <task> [args]: a built-in task, or one of charter.toml's [run.tasks]. Alone, it lists them.
func Run(args []string) {
	p := load()
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("charter run <task> [arguments]\n\nBuilt in:")
		for _, t := range tasks {
			fmt.Printf("  %-15s %s\n", t.name, t.about)
		}
		if len(p.Run.Tasks) > 0 {
			fmt.Println("\nThis app's (charter.toml's [run.tasks]):")
			names := make([]string, 0, len(p.Run.Tasks))
			for name := range p.Run.Tasks {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Printf("  %-11s %s\n", name, cmpOr(p.Run.Tasks[name].About, p.Run.Tasks[name].Command))
			}
		}
		return
	}
	name, rest := args[0], args[1:]
	if own, ok := p.Run.Tasks[name]; ok {
		do := func(base string) error {
			words := strings.Fields(strings.ReplaceAll(own.Command, "{base}", base))
			if len(words) == 0 {
				return fmt.Errorf("charter.toml's [run.tasks.%s] has no command", name)
			}
			if words[0] == "charter" {
				words[0] = self()
			}
			env := map[string]string{}
			if base != "" {
				env["BASE"] = base
			}
			return run.Run(p.Root, env, words[0], append(words[1:], rest...)...)
		}
		if own.Worker {
			check(serving(p, do))
		} else {
			check(do(""))
		}
		return
	}
	for _, t := range tasks {
		if t.name == name {
			t.run(p, rest)
			return
		}
	}
	check(fmt.Errorf("no task %s: charter run lists them", name))
}

// bench is charter run bench: the Worker on wrangler dev, each path asked for n times, one at a time, and how long an
// answer took: the median and the slowest tenth. It is the time a request waits on this machine, for comparing one
// build with another; it is not what Cloudflare bills.
func bench(p *project.Project, args []string) {
	fs := flag.NewFlagSet("charter run bench", flag.ExitOnError)
	n := fs.Int("n", 100, "requests per path")
	fs.Parse(args)
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{p.HealthPath()}
	}
	check(serving(p, func(base string) error {
		fmt.Printf("\n%-40s %10s %10s\n", "path ("+fmt.Sprint(*n)+" requests)", "median", "p90")
		for _, path := range paths {
			took := make([]time.Duration, 0, *n)
			for range *n {
				start := time.Now()
				if err := get(base + path); err != nil {
					return err
				}
				took = append(took, time.Since(start))
			}
			slices.Sort(took)
			fmt.Printf("%-40s %8.1f ms %8.1f ms\n", path, ms(took[len(took)/2]), ms(took[len(took)*9/10]))
		}
		return nil
	}))
}

// get asks for url and reads the whole answer; a status of 500 or more is an error.
func get(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return nil
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// generated are what a build writes, in an app: charter cache clean removes them.
var generated = []string{"build", "dist", ".wrangler", ".charter", filepath.Join("sdk", "out")}

// modulePath is how a Go module path looks beside an npm package name: a host with a dot, then a path.
var modulePath = regexp.MustCompile(`^[a-z0-9.-]+\.[a-z]+/`)

// uiTheme is charter run ui:theme: gsxui's own apply, which writes gsxui.preset.json and web/gsxui/theme.css and, for
// another style, vendors every component again in it. With no preset it says what the app has and where one is
// made. The pages follow at once under charter dev; the theme-toggle screen switches between its light and dark.
func uiTheme(p *project.Project, a []string) {
	var preset string
	var flags []string
	for i := 0; i < len(a); i++ {
		switch {
		case a[i] == "--only" || a[i] == "-only" || a[i] == "--preset" || a[i] == "-preset":
			if i+1 < len(a) {
				if strings.HasSuffix(a[i], "preset") {
					preset = a[i+1]
				} else {
					flags = append(flags, "--only", a[i+1])
				}
				i++
			}
		case strings.HasPrefix(a[i], "-"):
			flags = append(flags, a[i]) // --overwrite
		default:
			preset = a[i]
		}
	}
	if preset == "" {
		var now struct{ Style, Radius, FontSans string }
		b, err := os.ReadFile(filepath.Join(p.Root, "gsxui.preset.json"))
		if err != nil {
			check(errors.New("no gsxui.preset.json: the app has no gsxui yet (go tool gsxui init)"))
		}
		check(json.Unmarshal(b, &now))
		say("style %s, radius %s, font %s (gsxui.preset.json; the colours are web/gsxui/theme.css)\n\nAnother: make one at https://ui.gsxhq.dev/theme, then\n  go tool charter run ui:theme '<its share code>'\n  go tool charter run ui:theme my-preset.json --only theme", now.Style, now.Radius, now.FontSans)
		return
	}
	// --yes: gsxui asks before it rewrites; a command an agent or a script runs cannot answer.
	check(run.Run(p.Root, nil, "go", append([]string{"tool", "gsxui", "apply", "--preset", preset, "--yes"}, flags...)...))
	check(generate(p))
}
