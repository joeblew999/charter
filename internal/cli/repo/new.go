package repo

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/version"
)

// examples are what charter new makes a project from: start, one route; notes, the notes API (sign-in, streams, rate
// limits). Each is in TypeScript and in Go, in charter's repo (examples/<name>-ts, examples/<name>-go).
var examples = []string{"start", "notes"}

// left are what a copy leaves out: what npm, charter's local runner (.charter), Fern and the build write.
var left = []string{"node_modules", ".charter", "build", "package-lock.json", ".DS_Store"}

// LibURL is charter-lib on a release of charter, by version: what a TypeScript project's package.json installs.
func LibURL(v string) string {
	return fmt.Sprintf("https://github.com/joeblew999/charter/releases/download/v%s/charter-lib-%s.tgz", v, v)
}

// copyProject copies a project's own files from one folder to another, less those also names (paths in it).
func copyProject(from, to string, also []string) error {
	return filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		rel = filepath.ToSlash(rel)
		name := d.Name()
		if rel != "." && (slices.Contains(left, name) || rel == "sdk/out" || slices.Contains(also, rel) ||
			!d.IsDir() && (strings.HasPrefix(name, ".dev.vars") || strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".tsbuildinfo"))) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(to, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
}

// copyExample copies an example of charter's repo (at repo) into to: a TypeScript one as it is; a Go one with its
// TypeScript twin's tests, which in the repo it runs where they are, and the schemas they check its answers against,
// so that it stands alone. Its committed Go SDK is the example's (a project publishes its own), and so is the soak
// test of a redeploy (test/soak*), which charter runs by hand.
func copyExample(repo, example string, golang bool, to string) error {
	twin := filepath.Join(repo, "examples", example+"-ts")
	if !golang {
		return copyProject(twin, to, nil)
	}
	if err := copyProject(filepath.Join(repo, "examples", example+"-go"), to, []string{"sdk/go", "test/soak.ts", "test/soak-go"}); err != nil {
		return err
	}
	if err := copyProject(filepath.Join(twin, "test"), filepath.Join(to, "test"), nil); err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(twin, "api", "schemas.ts"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(to, "test", "schemas.ts"), b, 0o644); err != nil {
		return err
	}
	if err := rewrite(filepath.Join(to, "test"), func(c, _ string) string {
		return strings.ReplaceAll(c, `from "../api/schemas.ts"`, `from "./schemas.ts"`)
	}); err != nil {
		return err
	}
	// A Go project is charter's (charter.toml): its tests run from its own test/ now.
	if err := rewrite(to, func(c, file string) string {
		if filepath.Base(file) == "charter.toml" {
			return strings.ReplaceAll(c, "../"+example+"-ts/test", "test")
		}
		return c
	}); err != nil {
		return err
	}
	// Its tests are TypeScript, typechecked by charter check (tsc -p tsconfig.node.json), as its twin's are.
	var tsconfig map[string]any
	b, err = os.ReadFile(filepath.Join(twin, "tsconfig.node.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &tsconfig); err != nil {
		return err
	}
	tsconfig["include"] = []string{"test"}
	out, _ := json.MarshalIndent(tsconfig, "", "\t")
	return os.WriteFile(filepath.Join(to, "tsconfig.node.json"), append(out, '\n'), 0o644)
}

var textFile = regexp.MustCompile(`\.(ts|mts|js|mjs|json|ya?ml|toml|md|sql|html|css|txt|go)$|^go\.mod$|^\.?gitignore$`)

// rewrite changes every text file under dir.
func rewrite(dir string, change func(content, file string) string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !textFile.MatchString(d.Name()) {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if after := change(string(b), p); after != string(b) {
			return os.WriteFile(p, []byte(after), 0o644)
		}
		return nil
	})
}

// examplesRepo is the repo the examples are copied from, and what removes it when done: CHARTER_SOURCE (an archive of
// charter's files, or a checkout of them, as charter's test of new gives); charter's own checkout, when charter runs in
// one; else charter's repo at this release's tag, from GitHub.
func examplesRepo() (string, func(), error) {
	none := func() {}
	if r, err := Root(); err == nil && IsCharter(r) && os.Getenv("CHARTER_SOURCE") == "" {
		return r, none, nil
	}
	scratch, err := os.MkdirTemp("", "charter-examples-")
	if err != nil {
		return "", none, err
	}
	done := func() { os.RemoveAll(scratch) }
	archive := os.Getenv("CHARTER_SOURCE")
	if info, err := os.Stat(archive); err == nil && info.IsDir() {
		done()
		return archive, none, nil
	}
	if archive == "" {
		if !version.Released() {
			done()
			return "", none, errors.New("a charter built from a checkout makes a project from that checkout: run it in charter's repo, or set CHARTER_SOURCE")
		}
		v := strings.TrimPrefix(version.String(), "v")
		url := "https://github.com/joeblew999/charter/archive/refs/tags/v" + v + ".tar.gz"
		resp, err := http.Get(url)
		if err != nil || resp.StatusCode != 200 {
			done()
			return "", none, fmt.Errorf("the examples of charter %s could not be fetched from %s: %v", v, url, err)
		}
		defer resp.Body.Close()
		archive = filepath.Join(scratch, "charter.tar.gz")
		f, err := os.Create(archive)
		if err != nil {
			done()
			return "", none, err
		}
		_, err = io.Copy(f, resp.Body)
		f.Close()
		if err != nil {
			done()
			return "", none, err
		}
	}
	into := filepath.Join(scratch, "repo")
	if err := os.MkdirAll(into, 0o755); err != nil {
		done()
		return "", none, err
	}
	abs, _ := filepath.Abs(archive)
	if _, err := run.Output(into, nil, "tar", "-xzf", abs); err != nil {
		done()
		return "", none, err
	}
	// GitHub's archive has the repo in one folder (charter-<version>); one of the files alone has none.
	top, _ := os.ReadDir(into)
	if len(top) == 1 && top[0].IsDir() && !exists(filepath.Join(into, "examples")) {
		into = filepath.Join(into, top[0].Name())
	}
	return into, done, nil
}

func screaming(name string) string { return strings.ToUpper(strings.ReplaceAll(name, "-", "_")) }

// startJob is the CI a new project starts with, its check on every push and pull request.
const startJob = `
# This project's CI: charter init writes a workflow per job (.github/workflows/self-<job>.yml); charter ci <job> runs one
# here, the same way.
[ci.jobs.check]
about = "Every check that needs no Cloudflare account: charter check"
on = ["push", "pull_request"]
run = ["charter check"]
timeout = 30
`

// New runs charter new <name>: a new project, a copy of one of charter's tested examples, in TypeScript or (-go) in Go,
// under its own name, in a repo of its own that charter keeps.
func New(args []string) {
	fs := flag.NewFlagSet("charter new", flag.ExitOnError)
	from := fs.String("from", "start", "the example: start (one route) or notes (sign-in, streams, rate limits)")
	golang := fs.Bool("go", false, "in Go (Huma), not TypeScript (oRPC)")
	subdomain := fs.String("subdomain", "", "your workers.dev subdomain, for its URL")
	into := fs.String("into", "", "the folder (default: the name)")
	fs.Parse(flagsFirst(args))
	name := fs.Arg(0)
	if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(name) {
		check(errors.New("new needs a name of lowercase letters, digits and -, which is the Worker's name too: charter new my-api"))
	}
	if !slices.Contains(examples, *from) {
		check(fmt.Errorf("-from is one of %s", strings.Join(examples, ", ")))
	}
	dir := *into
	if dir == "" {
		dir = name
	}
	dir, err := filepath.Abs(dir)
	check(err)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		check(fmt.Errorf("%s is not empty", dir))
	}
	check(os.MkdirAll(dir, 0o755))
	language := "ts"
	if *golang {
		language = "go"
	}
	src, done, err := examplesRepo()
	check(err)
	err = copyExample(src, *from, *golang, dir)
	done()
	check(err)
	// The release this charter is; a checkout's charter has none, so the project says v0.0.0 until it takes one.
	v := "0.0.0"
	if version.Released() {
		v = strings.TrimPrefix(version.String(), "v")
	}
	old := "charter-" + *from + "-" + language
	sub := *subdomain
	if sub == "" {
		sub = "your-subdomain"
	}
	check(rewrite(dir, func(c, file string) string {
		c = strings.NewReplacer(old, name, screaming(old), screaming(name), ".gedw99.workers.dev", "."+sub+".workers.dev",
			"github.com/joeblew999/charter/examples/"+*from+"-go", name).Replace(c)
		switch {
		case filepath.Base(file) == "go.mod":
			// The Go library is the release's, and charter itself comes from mise, not go tool.
			c = regexp.MustCompile(`\nreplace github\.com/joeblew999/charter => \S+\n`).ReplaceAllString(c, "\n")
			c = regexp.MustCompile(`\ntool github\.com/joeblew999/charter/cmd/charter\n`).ReplaceAllString(c, "\n")
			c = strings.Replace(c, "github.com/joeblew999/charter v0.0.0", "github.com/joeblew999/charter v"+v, 1)
		case file == filepath.Join(dir, "package.json"):
			c = regexp.MustCompile(`"charter-lib": "[^"]*"`).ReplaceAllString(c, `"charter-lib": "`+LibURL(v)+`"`)
		case file == filepath.Join(dir, "charter.toml"):
			c = strings.TrimRight(c, "\n") + "\n" + startJob
		}
		return c
	}))
	what := map[bool]string{true: "Go", false: "TypeScript"}[*golang]
	fmt.Printf("made %s, from the %s example in %s, as the Worker %s\n", dir, *from, what, name)
	// Go: the imports sorted again (the module's new name sorts elsewhere), and the Go library of the release, fetched
	// (go.sum).
	tidy := true
	if *golang {
		check(run.Run(dir, nil, "gofmt", "-w", "."))
		if _, err := run.Output(dir, map[string]string{"GOWORK": "off"}, "go", "mod", "tidy"); err != nil {
			tidy = false
			fmt.Printf("go mod tidy failed: the Go library v%s could not be fetched. Run it again in %s when you can reach GitHub.\n", v, dir)
		}
	}
	// A project is a repo of its own, kept as every repo is: charter's files, its CI.
	if !exists(filepath.Join(dir, ".git")) {
		check(exec.Command("git", "-C", dir, "init", "-q").Run())
	}
	check(Init(dir))
	rel, err := filepath.Rel(".", dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = dir
	}
	fmt.Printf("\nnext:\n  cd %s\n", rel)
	if !tidy {
		fmt.Println("  go mod tidy          # the Go library")
	}
	fmt.Println("  npm install          # the packages")
	fmt.Println("  mise install         # charter, and what it runs (mise.toml)")
	fmt.Println("  charter check        # every check that needs no Cloudflare account")
	fmt.Println("  charter dev          # the API on charter.toml's port")
	if *subdomain == "" {
		fmt.Println("Your workers.dev subdomain goes in charter.toml's url (charter deploy prints it; your-subdomain for now), then charter spec.")
	}
}

// flagsFirst is args with its flags (-x, and -x value) before the rest, which Go's flag package stops at: charter new
// my-api -go is charter new -go my-api.
func flagsFirst(args []string) []string {
	takes := map[string]bool{"-from": true, "-subdomain": true, "-into": true, "--from": true, "--subdomain": true, "--into": true}
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case takes[a] && i+1 < len(args):
			flags = append(flags, a, args[i+1])
			i++
		case strings.HasPrefix(a, "-"):
			flags = append(flags, a)
		default:
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}
