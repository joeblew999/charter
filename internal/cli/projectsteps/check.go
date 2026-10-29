package projectsteps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/cli/scaffold"
	"github.com/joeblew999/charter/internal/fern"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/project"
)

// bundleLimit is what Workers Free allows a Worker, gzipped as a deploy uploads it: the Wasm the build makes is held
// to it.
const bundleLimit = 3_000_000

// Check runs charter check: every problem with charter.toml and charter's files in the app; then, for a project
// (project.IsProject), every check that needs no Cloudflare account, each timed: lint, go test, the specs against the
// contract, Fern's check, the app tried natively ([test] native), the Worker built and tried under workerd.
func Check(args []string) {
	fs := flag.NewFlagSet("charter check", flag.ExitOnError)
	fresh := fs.Bool("fresh", false, "build the Worker and run Fern's check again, even when charter's cache has them")
	fs.Parse(args)
	p := load()
	if !scaffold.Report(p) {
		os.Exit(1)
	}
	if p.IsProject() {
		check(projectCheck(p, *fresh))
	}
}

func projectCheck(p *project.Project, fresh bool) error {
	var times []string
	var total time.Duration
	timed := func(name string, part func() error) error {
		start := time.Now()
		if err := part(); err != nil {
			return err
		}
		took := time.Since(start)
		total += took
		times = append(times, name+" "+run.Seconds(took))
		return nil
	}
	if err := timed("lint", func() error { return lint(p) }); err != nil {
		return err
	}
	if err := timed("go test", func() error { return run.Run(p.Root, nil, "go", "test", "./...") }); err != nil {
		return err
	}
	if err := timed("specs", func() error { return spec(p, true) }); err != nil {
		return err
	}
	if exists(filepath.Join(p.Root, "fern")) {
		if err := timed("fern check", func() error { return fernCheck(p, fresh) }); err != nil {
			return err
		}
	}
	if p.Test.Native {
		if err := timed("native", func() error { return native(p) }); err != nil {
			return err
		}
	}
	if err := timed("build + workerd", func() error { return workerd(p, fresh) }); err != nil {
		return err
	}
	say("check took %s: %s", run.Seconds(total), strings.Join(times, ", "))
	return nil
}

// lint is a Go project's lint: its root's Go files and [go] lint's folders formatted, the root package vetted (for
// Wasm too, as the Worker is built), gsx's output and the committed Go SDK fresh, and its tests' TypeScript typed.
func lint(p *project.Project) error {
	files, _ := filepath.Glob(filepath.Join(p.Root, "*.go"))
	dirs := p.Go.Lint
	if len(dirs) == 0 {
		dirs = []string{"api"}
	}
	for _, d := range dirs {
		if exists(filepath.Join(p.Root, d)) {
			files = append(files, filepath.Join(p.Root, d))
		}
	}
	for i := range files {
		files[i], _ = filepath.Rel(p.Root, files[i])
	}
	out, err := run.Output(p.Root, nil, "gofmt", append([]string{"-l"}, files...)...)
	if err != nil {
		return err
	}
	if s := strings.TrimSpace(out); s != "" {
		return fmt.Errorf("not formatted (gofmt -w writes them as they should be):\n%s", s)
	}
	if err := run.Run(p.Root, nil, "go", "vet", "./..."); err != nil {
		return err
	}
	if err := run.Run(p.Root, map[string]string{"GOOS": "js", "GOARCH": "wasm"}, "go", "vet", "."); err != nil {
		return err
	}
	if p.Go.GSX {
		if err := gsxFresh(p.Root); err != nil {
			return err
		}
	}
	if p.SDKs.Go {
		if err := fern.GoSDKFresh(p.Root); err != nil {
			return err
		}
	}
	if exists(filepath.Join(p.Root, "tsconfig.node.json")) {
		return run.Run(p.Root, nil, "tsc", "-p", "tsconfig.node.json")
	}
	return nil
}

// notSources are the folders gsxFresh does not look in: what is installed or built.
var notSources = []string{"node_modules", "build", "dist", "target", "out"}

// generated is every *.x.go under root (gsx's output), by path, with its content.
func generated(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root && (strings.HasPrefix(d.Name(), ".") || slices.Contains(notSources, d.Name())) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".x.go") {
			b, err := os.ReadFile(path)
			out[path] = string(b)
			return err
		}
		return nil
	})
	return out, err
}

// gsxFresh runs gsx's generator and fails, naming them, if it changed a committed *.x.go: they were stale. What it
// wrote stays, so what to commit is there.
func gsxFresh(root string) error {
	before, err := generated(root)
	if err != nil {
		return err
	}
	if err := run.Run(root, nil, "go", "tool", "gsx", "generate", "--no-cache", "-q"); err != nil {
		return err
	}
	after, err := generated(root)
	if err != nil {
		return err
	}
	var changed []string
	for _, f := range slices.Sorted(maps.Keys(after)) {
		if b, ok := before[f]; !ok {
			changed = append(changed, f+" (new)")
		} else if b != after[f] {
			changed = append(changed, f)
		}
	}
	for _, f := range slices.Sorted(maps.Keys(before)) {
		if _, ok := after[f]; !ok {
			changed = append(changed, f+" (removed)")
		}
	}
	if len(changed) > 0 {
		return fmt.Errorf("gsx changed what it generates, so the committed files were stale. They are written now: commit them.\n  %s", strings.Join(changed, "\n  "))
	}
	say("gsx: the committed *.x.go are what the components generate")
	return nil
}

// fernCheck is Fern's check of the specs and its settings, skipped when it passed before for the same files in fern/
// and the same Fern: most of its time is Fern starting up.
func fernCheck(p *project.Project, fresh bool) error {
	h := sha256.New()
	if version := fernVersion(p.Root); version != "" {
		fmt.Fprintf(h, "fern-api %s\n", version)
	}
	err := filepath.WalkDir(filepath.Join(p.Root, "fern"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(p.Root, path)
		b, err := os.ReadFile(path)
		fmt.Fprintf(h, "%s\n", filepath.ToSlash(rel))
		h.Write(b)
		return err
	})
	if err != nil {
		return err
	}
	cache, err := run.Cache("fern-check")
	if err != nil {
		return err
	}
	stamp := filepath.Join(cache, hex.EncodeToString(h.Sum(nil))[:24])
	if !fresh && exists(stamp) {
		say("fern check: passed before for these specs and settings (charter's cache); -fresh runs it again")
		return nil
	}
	if err := run.Run(p.Root, nil, "fern", "check", "--warnings"); err != nil {
		return err
	}
	return os.WriteFile(stamp, nil, 0o644)
}

// fernVersion is the version of the Fern the project installs (fern-api, in node_modules here or a workspace above).
func fernVersion(root string) string {
	for d := root; ; d = filepath.Dir(d) {
		if b, err := os.ReadFile(filepath.Join(d, "node_modules", "fern-api", "package.json")); err == nil {
			var pkg struct{ Version string }
			json.Unmarshal(b, &pkg)
			return pkg.Version
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

// helpers starts what the tests serve ([test] serve), each up before the next, then runs body, and stops them after.
func helpers(p *project.Project, pl places, body func() error) error {
	inner := body
	for i := len(p.Test.Serve) - 1; i >= 0; i-- {
		h, next := p.Test.Serve[i], inner
		inner = func() error {
			words, err := pl.command(h.Command)
			if err != nil {
				return err
			}
			return run.With(run.Server{Command: words, Dir: p.Root, Env: pl.env(p), Ready: pl.fill(h.Ready)}, next)
		}
	}
	return inner()
}

// native tries the app run natively (go run .) on a free port, with [local] vars and [test] env: the standard test,
// then [test] run.
func native(p *project.Project) error {
	ports, err := run.FreePorts(4)
	if err != nil {
		return err
	}
	pl := places{base: "http://localhost:" + strconv.Itoa(ports[0]), ports: ports[1:]}
	return helpers(p, pl, func() error {
		env := localEnv(p, pl)
		env["PORT"] = strconv.Itoa(ports[0])
		return run.With(run.Server{Command: []string{"go", "run", "."}, Dir: p.Root, Env: env, Ready: pl.base + "/api/openapi.json"},
			func() error { return try(p, p.Test.Run, pl) })
	})
}

// workerd builds the Worker (TinyGo, within what Workers Free allows) and tries it on local workerd (charter with,
// which fails on a panic in its log too): the standard test, then [test] run, as charter test run runs them there.
func workerd(p *project.Project, fresh bool) error {
	build := []string{"build", "-limit=" + strconv.Itoa(bundleLimit)}
	if fresh {
		build = append(build, "-fresh")
	}
	if err := run.Run(p.Root, nil, self(), build...); err != nil {
		return err
	}
	ports, err := run.FreePorts(3)
	if err != nil {
		return err
	}
	pl := places{ports: ports}
	return helpers(p, pl, func() error {
		env := pl.env(p)
		var s []string
		for _, n := range ports {
			s = append(s, strconv.Itoa(n))
		}
		env[portsVar] = strings.Join(s, ",")
		err := run.Run(p.Root, env, self(), "with", "-strict", self(), "test", "run")
		if err != nil {
			return errors.New("the Worker under workerd failed its tests (above)")
		}
		return nil
	})
}
