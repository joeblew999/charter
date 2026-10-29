// Package repo is how charter keeps a repo: its files (charter init writes them, charter check says when one is stale),
// the repo on GitHub (charter repo), its jobs here and there (charter ci <job>), its issues and the upstream issues its
// code works around, its docs, a new project (charter new), and charter's own release (charter release).
//
//	charter issues [bug|feature|upstream|plan]   the open issues; with a kind, a new one's body and the gh command
//	charter upstream                             the upstream issues the code works around (Upstream: tags), their state
//	charter repo [-check]                        the repo on GitHub as charter.toml's [github] says
//	charter docs [-review [-print]]              what a program can see wrong in docs/; -review: Claude reads the rest
//	charter ci [<job> [<project>]] [-remote]     a job of charter.toml's [ci.jobs] here, as GitHub runs it
//	charter new <name> [-from start|notes] [-go] a new project, a copy of a tested example, in a repo of its own
//	charter release <X.Y.Z> [-dry-run]           a release of charter itself, from its own repo
//
// A repo's settings are charter.toml at its top: [github] and [ci] (go doc github.com/joeblew999/charter/project).
package repo

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/project"
)

// Command is one of charter's commands, as docs/charter/README.md and charter help say it.
type Command struct {
	Name, Usage string
	When        string // when a developer runs it; "" for one docs/charter/README.md leaves out
	About       string
	Project     bool // a project's step (in a folder with a charter.toml naming a Worker), not the repo's
}

// Commands are charter's commands, in the order a developer meets them. cmd/charter runs each (a test holds the two
// alike).
var Commands = []Command{
	{"issues", "issues [bug|feature|upstream|plan]", "First, whenever you start; with a kind, to file a bug or a plan", "REMOTE, read-only: the open issues, newest first; with a kind, the body of a new one and the gh command that files it", false},
	{"check", "check [-fresh] [-go]", "Before every commit", "every check that needs no Cloudflare account: charter's files; in a project, its types, its specs against the contract, its tests, the Worker run and tested, its size; at a repo's top, the jobs charter.toml's [ci] check lists; -go: the folder's Go module (gofmt, go vet for Wasm too, go test)", false},
	{"docs", "docs [-review [-print]]", "After changing a page in docs/", "what a program can see wrong in docs/ (front matter, links and anchors, pages nothing links to, commands, scripts and paths that don't exist, pinned release versions); -review has Claude read it for the rest", false},
	{"ci", "ci [<job> [<project>]] [-remote]", "To run a job as GitHub does, here; -remote to start it there", "a job of charter.toml's [ci.jobs] here, as GitHub runs it; with none, every job GitHub runs on a change (an app with no jobs: its [ci], charter ci run); -remote starts the job on GitHub instead", false},
	{"upstream", "upstream", "Now and then: a CLOSED one means its workaround can go", "REMOTE, read-only: the upstream issues the code works around (its Upstream: tags), and their state", false},
	{"repo", "repo [-check]", "After changing charter.toml's [github]", "REMOTE: the repo on GitHub as charter.toml's [github] says: labels, description, homepage, topics, GitHub Pages (a public repo's); -check changes nothing and fails on drift", false},
	{"init", "init [-check] [-name n] [-compat date]", "After installing another release of charter, and after changing charter.toml's [ci] or [github]", "write charter's files into the repo (never one of the repo's own), its mise tasks and skill, and a workflow per job of charter.toml; -check writes nothing and fails if one differs", false},
	{"dev", "dev [-native]", "To try the API on your machine", "the Worker on your machine, as on Cloudflare (workerd), its local database migrated, until Ctrl-C; -native, in Go: go run .", true},
	{"spec", "spec [-check]", "After every change to the contract, to charter.toml's [sdks], or to migrations/ or api/queries.sql", "write both specs (fern/openapi.json, fern/asyncapi.json) from the contract, Fern's settings (fern/generators.yml) from them and [sdks], and the database's typed code from migrations/ (db: sqlc in Go, drizzle in TypeScript)", true},
	{"sdk", "sdk [<group>] [-prove [<group>…]] [-publish]", "When a caller needs an SDK; -prove after changing [sdks]; -publish after a contract change, where the repo commits its Go SDK", "an SDK with Fern (Docker), by default the TypeScript one, into sdk/out; -prove: every client, proven in its language; -publish, in Go: the Go SDK into sdk/go", true},
	{"build", "build", "", "the Worker into build/: Go with TinyGo, TypeScript bundled with esbuild", true},
	{"deploy", "deploy production|staging|preview <name>", "To put the API on Cloudflare", "REMOTE: the Worker on Cloudflare with its secrets (fnox exec -- charter deploy production), then its migrations", true},
	{"live-test", "live-test", "After every deploy", "REMOTE: the standard test and the project's tests against the deployed Worker", true},
	{"new", "new <name> [-from start|notes] [-go] [-subdomain s] [-into dir]", "", "a new project, a copy of a tested example under your name, in TypeScript or (-go) in Go, in a repo of its own", false},
	{"twins", "twins <reference> <other>", "", "a TypeScript project and its Go twin under workerd, the same requests to both, failing if an answer differs", false},
	{"release", "release <X.Y.Z> [-dry-run]", "", "a release of charter itself, from its own repo: the check, charter-lib packed, the binaries, the tag vX.Y.Z", false},
}

// Root is the top of the git repo charter runs in.
func Root() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", errors.New("this runs in a git repo (git init first)")
	}
	return strings.TrimSpace(string(out)), nil
}

// AtTop is the top of the git repo, and whether the working directory is it (each with its links resolved: a Mac's
// temporary folders are under a link).
func AtTop() (string, bool) {
	top, err := Root()
	if err != nil {
		return "", false
	}
	here, _ := os.Getwd()
	a, errA := filepath.EvalSymlinks(here)
	b, errB := filepath.EvalSymlinks(top)
	return top, errA == nil && errB == nil && a == b
}

// IsCharter is whether root is charter's own repo (its go.mod is the module github.com/joeblew999/charter).
func IsCharter(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	return err == nil && regexp.MustCompile(`(?m)^module\s+github\.com/joeblew999/charter\s*$`).Match(b)
}

// settings is the repo's charter.toml, at its top.
func settings(root string) (*project.Project, error) { return project.LoadRepo(root) }

// gh runs gh in root and returns what it printed.
func gh(root string, args ...string) (string, error) { return run.Output(root, nil, "gh", args...) }

// say prints a line as adopt's report did: what was done, and to what.
func say(did, what, why string) { fmt.Printf("%-7s  %s%s\n", did, what, why) }

func check(err error) {
	if err != nil {
		log.Fatal("✗ ", err)
	}
}

func root() string {
	log.SetFlags(0)
	r, err := Root()
	check(err)
	return r
}
