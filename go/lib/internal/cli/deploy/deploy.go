// Package deploy is go tool charter deploy (go tool workerdeploy is the same command): it deploys a Go Worker to Cloudflare with the REST API, no wrangler and no Node: the build
// workerbuild wrote, its static assets, bindings (plain text, D1, Durable Objects, rate limiting), D1 and Durable
// Object migrations and custom domains (internal/cfdeploy documents the steps). It also deploys and deletes Worker
// Previews, and runs the preview pipeline that hands CI's build to Workers Builds (internal/previewbuild).
//
// Credentials come from CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID, e.g. with `fnox exec --`. It prints the
// Worker's workers.dev URL (or, with -preview, the preview's URL) on stdout and progress on stderr. -preview NAME
// deploys a Worker Preview instead of production; -delete-preview NAME removes one.
package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/joeblew999/charter/go/lib/internal/cfdeploy"
	"github.com/joeblew999/charter/go/lib/internal/previewbuild"
)

// previewTag is the pre-release that carries CI's builds for previews.
const previewTag = "preview-builds"

type list []string

func (l *list) String() string     { return strings.Join(*l, ",") }
func (l *list) Set(s string) error { *l = append(*l, s); return nil }

// rateLimits parses NAME=NAMESPACE_ID:LIMIT/PERIOD, e.g. WRITES=1001:60/10.
type rateLimits map[string]cfdeploy.RateLimit

func (r rateLimits) String() string { return fmt.Sprint(map[string]cfdeploy.RateLimit(r)) }
func (r rateLimits) Set(s string) error {
	var rl cfdeploy.RateLimit
	name, spec, ok := strings.Cut(s, "=")
	ns, window, ok2 := strings.Cut(spec, ":")
	if !ok || !ok2 || name == "" {
		return fmt.Errorf("want NAME=NAMESPACE_ID:LIMIT/PERIOD, got %q", s)
	}
	if _, err := fmt.Sscanf(window, "%d/%d", &rl.Limit, &rl.Period); err != nil {
		return fmt.Errorf("want NAME=NAMESPACE_ID:LIMIT/PERIOD, got %q", s)
	}
	rl.NamespaceID = ns
	r[name] = rl
	return nil
}

type vars map[string]string

func (v vars) String() string { return fmt.Sprint(map[string]string(v)) }
func (v vars) Set(s string) error {
	k, val, ok := strings.Cut(s, "=")
	if !ok || k == "" {
		return fmt.Errorf("want KEY=VALUE, got %q", s)
	}
	v[k] = val
	return nil
}

// Main runs the command with args, the arguments after its name.
func Main(args []string) {
	fs := flag.NewFlagSet("charter deploy", flag.ExitOnError)
	log.SetFlags(0)
	cfg := cfdeploy.Config{Vars: vars{}, D1: vars{}, DurableObjects: vars{}, RateLimits: rateLimits{}}
	fs.StringVar(&cfg.Name, "name", "", "Worker script name (required)")
	fs.StringVar(&cfg.BuildDir, "build", "build", "directory with worker.mjs, wasm_exec.js, runtime.mjs, app.wasm")
	fs.StringVar(&cfg.AssetsDir, "assets", "", `static assets directory ("" for none)`)
	fs.StringVar(&cfg.CompatibilityDate, "compat", "", "compatibility_date (required): the one the Worker is tested with")
	fs.BoolVar(&cfg.AllowExisting, "allow-existing", false, "update the Worker if a script with this name already exists")
	fs.StringVar(&cfg.MainModule, "main", "", `entry module that imports ./build/worker.mjs, e.g. worker/index.mjs ("" = build/worker.mjs is the entry)`)
	fs.BoolVar(&cfg.D1Create, "d1-create", false, "create D1 databases named in -d1 that don't exist yet")
	fs.StringVar(&cfg.MigrationsDir, "migrations", "", "apply these *.sql files (in name order) to every -d1 database")
	fs.StringVar(&cfg.MigrationTag, "migration-tag", "", "Durable Object migration tag to reach (sent only if not yet applied)")
	fs.Var(vars(cfg.Vars), "var", "plain_text binding KEY=VALUE (repeatable)")
	fs.Var(vars(cfg.D1), "d1", "D1 binding NAME=DATABASE_NAME or NAME=DATABASE_ID (repeatable)")
	fs.Var(vars(cfg.DurableObjects), "do", "Durable Object binding NAME=CLASS (repeatable)")
	fs.Var(rateLimits(cfg.RateLimits), "ratelimit", "rate limiting binding NAME=NAMESPACE_ID:LIMIT/PERIOD, e.g. WRITES=1001:60/10 (repeatable)")
	fs.Var((*list)(&cfg.Domains), "domain", "Workers Custom Domain to attach, e.g. app.example.com (repeatable; zone must be in the account)")
	fs.Var((*list)(&cfg.Modules), "module", "extra ES module next to -main, e.g. worker/room.mjs (repeatable)")
	fs.Var((*list)(&cfg.NewSQLiteClasses), "new-sqlite-class", "Durable Object class created by -migration-tag (repeatable)")
	dryRun := fs.Bool("dry-run", false, "print what would be uploaded; make no API calls")
	previewName := fs.String("preview", "", "deploy as this Worker Preview of -name (e.g. pr-7) instead of production; prints its URL")
	deletePreview := fs.String("delete-preview", "", "delete this Worker Preview of -name and exit")
	annotations := vars{}
	fs.Var(annotations, "annotation", "preview deployment annotation KEY=VALUE, e.g. workers/pull_request_number=7 (repeatable)")
	// The preview pipeline (internal/previewbuild):
	repo := fs.String("repo", os.Getenv("APP_REPO"), "GitHub owner/name for the preview-build modes (default $APP_REPO)")
	publishBuild := fs.String("publish-build", "", "CI: publish this commit's build (build/, -assets, this binary) to the preview-builds pre-release (GITHUB_TOKEN)")
	fetchBuild := fs.String("fetch-build", "", "fetch and verify this commit's build from the preview-builds pre-release into the current directory (exit 3 if it never appears)")
	wait := fs.Duration("wait", 8*time.Minute, "how long -fetch-build waits for CI to publish")
	slug := fs.String("slug", "", "print the preview name for this branch and exit")
	prunePreviews := fs.Bool("prune-previews", false, "delete -name's previews whose branch no longer exists on GitHub")
	commentPR := fs.String("comment-pr", "", "CI: post the preview URL (-url) on this branch's open pull request (GITHUB_TOKEN)")
	commentURL := fs.String("url", "", "preview URL for -comment-pr")
	commentNote := fs.String("comment-note", "", "-comment-pr: a sentence about the app's previews, after the URL (what they share, whether they're indexed)")
	configureBuilds := fs.Bool("configure-builds", false, "make -name's Workers Builds setup match the flags below (needs Cloudflare credentials)")
	bc := cfdeploy.BuildsConfig{Vars: vars{}}
	fs.StringVar(&bc.BuildToken, "build-token", "", "-configure-builds: Workers Builds token name (make it once in the dashboard)")
	fs.StringVar(&bc.FallbackBuildToken, "fallback-build-token", "", "-configure-builds: token name to use until -build-token exists")
	fs.StringVar(&bc.BuildCommand, "build-command", "", "-configure-builds: the build command Workers Builds runs (required)")
	fs.StringVar(&bc.DeployCommand, "deploy-command", "", "-configure-builds: the deploy command Workers Builds runs (required)")
	fs.Var(vars(bc.Vars), "build-var", "-configure-builds: build variable KEY=VALUE (repeatable)")
	fs.Var((*list)(&bc.PathExcludes), "path-exclude", "-configure-builds: don't build pushes that only change these paths (repeatable)")
	fs.Parse(args)

	ctx := context.Background()
	gh := &previewbuild.GitHub{Repo: *repo, Token: os.Getenv("GITHUB_TOKEN"), Logf: log.Printf, HTTP: &http.Client{Timeout: 2 * time.Minute}}
	switch {
	case *slug != "":
		fmt.Println(previewbuild.Slug(*slug, cfg.Name))
		return
	case *publishBuild != "":
		need(*repo != "" && gh.Token != "", "-publish-build needs -repo (or APP_REPO) and GITHUB_TOKEN")
		paths := []string{cfg.BuildDir}
		if cfg.AssetsDir != "" {
			paths = append(paths, cfg.AssetsDir)
		}
		data, err := previewbuild.Pack(".", *publishBuild, paths...)
		check(err)
		check(gh.Publish(ctx, previewTag, *publishBuild+".tar.gz", data, 80))
		self, err := os.Executable()
		check(err)
		bin, err := os.ReadFile(self)
		check(err)
		check(gh.Publish(ctx, previewTag, "deploy-"+*publishBuild, bin, 80))
		return
	case *fetchBuild != "":
		need(*repo != "", "-fetch-build needs -repo (or APP_REPO)")
		data, ok, err := gh.Fetch(ctx, previewTag, *fetchBuild+".tar.gz", *wait, 10*time.Second)
		check(err)
		if !ok {
			log.Printf("✗ no build for %s after %s", *fetchBuild, *wait)
			os.Exit(3)
		}
		m, err := previewbuild.Unpack(bytes.NewReader(data), ".", *fetchBuild)
		check(err)
		log.Printf("✓ build for %s: %d files, verified", *fetchBuild, len(m.Files))
		return
	case *commentPR != "":
		need(*repo != "" && gh.Token != "" && *commentURL != "", "-comment-pr needs -repo, GITHUB_TOKEN and -url")
		_, err := gh.CommentPR(ctx, *commentPR, "<!-- worker-preview -->",
			strings.TrimSpace("**Preview:** "+*commentURL+"\n\nDeployed by Workers Builds a few seconds after this commit's build is published; "+
				"its check goes green when it's live. "+*commentNote))
		check(err)
		return
	case *prunePreviews:
		need(*repo != "", "-prune-previews needs -repo (or APP_REPO)")
		branches, err := remoteBranches(*repo)
		check(err)
		live := map[string]bool{}
		for _, b := range branches {
			live[previewbuild.Slug(b, cfg.Name)] = true
		}
		_, err = client().PrunePreviews(ctx, cfg.Name, func(n string) bool { return live[n] })
		check(err)
		return
	case *configureBuilds:
		need(*repo != "", "-configure-builds needs -repo (or APP_REPO)")
		bc.Worker = cfg.Name
		bc.Owner, bc.Repo, _ = strings.Cut(*repo, "/")
		ids, err := repoIDs(*repo)
		check(err)
		bc.OwnerID, bc.RepoID = ids[0], ids[1]
		id, err := client().ConfigureBuilds(ctx, bc)
		check(err)
		log.Printf("✓ Workers Builds for %s: preview trigger %s", cfg.Name, id)
		return
	}

	if *deletePreview != "" {
		gone, err := client().DeletePreview(context.Background(), cfg.Name, *deletePreview)
		if err != nil {
			log.Fatalf("deploy: %v", err)
		}
		if !gone {
			log.Printf("preview %s of %s: none to delete", *deletePreview, cfg.Name)
		}
		return
	}

	plan, err := cfdeploy.NewPlan(cfg)
	if err != nil {
		log.Fatalf("deploy: %v (build first: go tool workerbuild)", err)
	}
	log.Printf("worker %q: %d modules (main %s), %d assets from %q, vars %v, d1 %v, do %v, ratelimit %v, domains %v, migration %q %v",
		cfg.Name, len(plan.Modules), plan.Modules[0].Name, len(plan.Assets), cfg.AssetsDir, cfg.Vars, cfg.D1, cfg.DurableObjects, cfg.RateLimits, cfg.Domains, cfg.MigrationTag, cfg.NewSQLiteClasses)
	if *dryRun {
		for _, m := range plan.Modules {
			log.Printf("  module %-22s ← %s", m.Name, m.Path)
		}
		for p, e := range plan.Assets {
			log.Printf("  asset  %-22s %s %dB", p, e.Hash, e.Size)
		}
		return
	}

	c := client()
	if *previewName != "" {
		res, err := c.DeployPreview(context.Background(), cfg, *previewName, annotations)
		if err != nil {
			log.Fatalf("deploy: %v", err)
		}
		for _, u := range res.URLs[min(1, len(res.URLs)):] {
			log.Printf("  deployment URL %s", u)
		}
		fmt.Println(res.URL())
		return
	}
	url, err := c.Deploy(context.Background(), cfg)
	if err != nil {
		log.Fatalf("deploy: %v", err)
	}
	fmt.Println(url)
}

// client is the API client, with credentials from CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID.
func client() *cfdeploy.Client {
	c := &cfdeploy.Client{
		Token:     os.Getenv("CLOUDFLARE_API_TOKEN"),
		AccountID: os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
		HTTP:      &http.Client{Timeout: 2 * time.Minute},
		Logf:      log.Printf,
	}
	if c.Token == "" || c.AccountID == "" {
		log.Fatal("deploy: CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID must be set (use fnox exec)")
	}
	return c
}

func need(ok bool, msg string) {
	if !ok {
		log.Fatal("deploy: " + msg)
	}
}

func check(err error) {
	if err != nil {
		log.Fatalf("deploy: %v", err)
	}
}

// remoteBranches lists the public repo's branches with git (no API rate limit, no token).
func remoteBranches(repo string) ([]string, error) {
	out, err := exec.Command("git", "ls-remote", "--heads", "https://github.com/"+repo).Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote %s: %w", repo, err)
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if _, ref, ok := strings.Cut(line, "\trefs/heads/"); ok {
			branches = append(branches, ref)
		}
	}
	if len(branches) == 0 {
		return nil, fmt.Errorf("git ls-remote %s: no branches (refusing to prune everything)", repo)
	}
	return branches, nil
}

// repoIDs returns the GitHub owner and repository ids (public API, no token).
func repoIDs(repo string) ([2]string, error) {
	resp, err := http.Get("https://api.github.com/repos/" + repo)
	if err != nil {
		return [2]string{}, err
	}
	defer resp.Body.Close()
	var r struct {
		ID    int64 `json:"id"`
		Owner struct {
			ID int64 `json:"id"`
		} `json:"owner"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil || r.ID == 0 {
		return [2]string{}, fmt.Errorf("github repo %s: %v (HTTP %d)", repo, err, resp.StatusCode)
	}
	return [2]string{fmt.Sprint(r.Owner.ID), fmt.Sprint(r.ID)}, nil
}
