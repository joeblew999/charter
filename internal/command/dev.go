package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	intlconfig "github.com/joeblew999/charter/intl/config"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Dev runs charter dev: the loop a developer works in, on the runtime the app ships on.
//
// It is gsx's own loop, go tool gsx dev, with its documented hooks: the build is charter build -dev (the Wasm) and
// the server is wrangler dev. So a save regenerates, rebuilds, restarts the Worker and reloads the browser through
// Vite, and what is on screen is workers-go on the Workers runtime with wrangler.jsonc's bindings, as in production.
// A mistake shows in the browser, in gsx's overlay, and the last page that worked stays up. Cmd-D (Ctrl-D) is gsx's
// dev panel, with the Worker's log.
//
// What gsx does not watch, charter does, ten times a second: the catalogs (locales/*.toml, charter.toml), the docs
// (docs/*.md) and the database (db/*.sql). Each is written as Go again, which gsx then builds; when one fails, the
// build fails with what is wrong, so that shows in the overlay too.
//
// Everything else a developer or an agent runs meanwhile, in another terminal, reaches the open page by itself.
// Where the loop is (its address, its log, its process) is in .charter/dev.json, which charter env reads.
func Dev(args []string) {
	if len(args) > 0 && args[0] == "-worker" {
		devWorker()
		return
	}
	p := load()
	if !hasGSX(p) {
		Preview(args)
		return
	}
	// A loop that was killed may have left its Worker running: stopped by its process id, which that loop wrote down.
	stopWorker(p)
	check(os.RemoveAll(devDir(p)))
	check(os.MkdirAll(devDir(p), 0o755))
	check(generate(p)) // the first build needs the messages and Vite's manifest
	check(migrate(p, map[string]string{"WRANGLER_SEND_METRICS": "false"}, "--local"))
	if c, err := intlconfig.Find(p.Root); err == nil && hasIntl(p) {
		go watch(p, "languages", "messages", func() []string {
			files, _ := filepath.Glob(filepath.Join(c.CatalogDir(), "*.toml"))
			return append(files, filepath.Join(p.Root, project.File))
		})
	}
	go watch(p, "database", "db", func() []string {
		return []string{filepath.Join(p.Root, "db", "schema.sql"), filepath.Join(p.Root, "db", "queries.sql"), filepath.Join(p.Root, "db", "seed.sql")}
	})
	go watch(p, "docs", "docs", func() []string {
		files, _ := filepath.Glob(filepath.Join(p.Root, "docs", "*.md"))
		return files
	})
	// As gsx's flags take them: a command line, split on spaces. go tool charter is this charter, by the app's go.mod.
	// The log is what the dev panel shows, and what charter env points at.
	gsx := []string{"tool", "gsx", "dev", "--build", "go tool charter build -dev", "--run", "go tool charter dev -worker", "--log-file", filepath.Join(devDir(p), "dev.log")}
	// The Worker runs on its own, so that a build does not restart it: whatever stops this process (Ctrl-C, which
	// the terminal tells gsx dev too; a parent ending it), gsx dev is stopped first and then the Worker.
	cmd := exec.Command("go", append(gsx, args...)...)
	cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = p.Root, os.Stdin, os.Stdout, os.Stderr
	check(cmd.Start())
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-stopped:
		cmd.Process.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			cmd.Process.Kill()
		}
	}
	stopWorker(p)
	os.RemoveAll(devDir(p))
	os.Remove(filepath.Join(p.Root, "poke.built.go"))
	check(err)
}

// stopWorker stops the wrangler dev that a charter dev started here, by the process id it wrote down.
func stopWorker(p *project.Project) {
	var state DevState
	if b, err := os.ReadFile(filepath.Join(devDir(p), "dev.json")); err == nil && json.Unmarshal(b, &state) == nil && run.Alive(state.PID) {
		run.KillGroup(state.PID)
		for range 50 {
			if !run.Alive(state.PID) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// builtDir is the folder wrangler.jsonc has wrangler watch (build's watch_dir): .charter/built, where charter dev
// writes which build is new and nothing else does, since every change in it is a build to wrangler.
func builtDir(p *project.Project) string { return filepath.Join(p.Root, ".charter", "built") }

// devDir is where a running charter dev keeps what others read of it: .charter/dev.
func devDir(p *project.Project) string { return filepath.Join(p.Root, ".charter", "dev") }

// DevState is a running charter dev, as .charter/dev/dev.json has it: what a developer's other commands and an agent
// read to find it.
type DevState struct {
	URL    string `json:"url"`    // the page: Vite's front door
	Worker string `json:"worker"` // wrangler dev itself, behind it
	Log    string `json:"log"`    // what the Worker and the builds print
	PID    int    `json:"pid"`    // wrangler dev's process
}

// devState is the charter dev running in this app, ok false when none is.
func devState(p *project.Project) (state DevState, ok bool) {
	b, err := os.ReadFile(filepath.Join(devDir(p), "dev.json"))
	if err != nil || json.Unmarshal(b, &state) != nil {
		return state, false
	}
	// A loop that was killed leaves its file: it is running when its Worker answers.
	client := http.Client{Timeout: time.Second}
	res, err := client.Get(state.Worker + p.HealthPath())
	if err != nil {
		return state, false
	}
	res.Body.Close()
	return state, true
}

// devWorker is charter dev -worker, what gsx dev runs after each build and stops before the next. The Worker itself,
// wrangler dev, is started once and left running across builds (in a process group of its own, so gsx stopping this
// process does not stop it): the build tells wrangler there is a new one, by the folder wrangler.jsonc has it watch
// (build's watch_dir, .charter/built), and wrangler swaps the new Worker in where it stands (swapWorker). That is what
// a save costs: the Wasm's build, and under a second. This process starts wrangler when it is not running, and
// passes on what it prints for as long as it lives.
//
// wrangler is told where Vite's dev server is (VITE_DEV_URL, which app.App reads as a variable of the Worker), on
// the port gsx chose (GO_PORT), and that the build is done (devBuilt).
func devWorker() {
	p := load()
	port := os.Getenv("GO_PORT")
	if port == "" {
		check(errors.New("charter dev -worker is what charter dev has gsx run: it needs GO_PORT, which gsx dev sets"))
	}
	worker := "http://127.0.0.1:" + port
	log := filepath.Join(devDir(p), "worker.log")
	check(os.MkdirAll(devDir(p), 0o755))
	from := int64(0)
	if info, err := os.Stat(log); err == nil {
		from = info.Size()
	}
	var state DevState
	b, _ := os.ReadFile(filepath.Join(devDir(p), "dev.json"))
	json.Unmarshal(b, &state)
	if state.Worker != worker || !run.Alive(state.PID) || !watchesDev(p) {
		if run.Alive(state.PID) {
			run.KillGroup(state.PID)
		}
		inspector, err := run.FreePort()
		check(err)
		bin, err := npm(p, "wrangler")
		check(err)
		// --test-scheduled: a Cron Trigger can be fired by hand, GET /__scheduled?cron=*+*+*+*+*
		args := []string{"dev", "--port", port, "--inspector-port", strconv.Itoa(inspector), "--show-interactive-dev-session=false", "--test-scheduled"}
		vite := os.Getenv("VITE_DEV_URL")
		if vite != "" {
			args = append(args, "--var", "VITE_DEV_URL:"+vite)
		}
		pid, err := run.Detached(p.Root, map[string]string{devBuilt: "1", "WRANGLER_SEND_METRICS": "false"}, log, bin, args...)
		check(err)
		state = DevState{URL: vite, Worker: worker, Log: filepath.Join(devDir(p), "dev.log"), PID: pid}
		b, _ := json.MarshalIndent(state, "", "  ")
		check(os.WriteFile(filepath.Join(devDir(p), "dev.json"), append(b, '\n'), 0o644))
	}
	// What wrangler prints, from here on, is this process's output: the terminal's, and the dev panel's log.
	go func() {
		for ; ; time.Sleep(200 * time.Millisecond) {
			f, err := os.Open(log)
			if err != nil {
				continue
			}
			if _, err := f.Seek(from, io.SeekStart); err == nil {
				n, _ := io.Copy(os.Stdout, f)
				from += n
			}
			f.Close()
		}
	}()
	for ; ; time.Sleep(time.Second) {
		if !run.Alive(state.PID) {
			check(errors.New("wrangler dev stopped: what it said is above"))
		}
	} // until gsx stops this process, before its next build's turn
}

// swapWorker has the Worker that charter dev left running take the build just made, and waits until the Worker
// answering is that build: charter build -dev ends with it, so when gsx dev sees the build done and reloads the
// browser, the page is the new one. Nothing to do when no Worker is running yet (the first build), or when
// wrangler.jsonc does not have wrangler watch .charter/built, where the Worker is started again instead.
func swapWorker(p *project.Project) error {
	var state DevState
	b, err := os.ReadFile(filepath.Join(devDir(p), "dev.json"))
	if err != nil || json.Unmarshal(b, &state) != nil || !run.Alive(state.PID) || !watchesDev(p) {
		return nil
	}
	want := buildID(p)
	if err := os.WriteFile(filepath.Join(builtDir(p), "id"), []byte(want), 0o644); err != nil {
		return err
	}
	client := http.Client{Timeout: time.Second}
	for start := time.Now(); ; time.Sleep(30 * time.Millisecond) {
		if res, err := client.Get(state.Worker + p.HealthPath()); err == nil {
			res.Body.Close()
			if res.Header.Get("X-Charter-Build") == want {
				return nil
			}
		}
		if !run.Alive(state.PID) {
			return errors.New("wrangler dev stopped while taking the build: its log is " + filepath.Join(devDir(p), "worker.log"))
		}
		if time.Since(start) > time.Minute {
			return fmt.Errorf("the Worker never became build %s: wrangler dev did not take it (its log: %s)", want, filepath.Join(devDir(p), "worker.log"))
		}
	}
}

// buildID names the build charter build -dev made last, which the Worker says back on /healthz.
func buildID(p *project.Project) string {
	b, _ := os.ReadFile(filepath.Join(p.Root, "build", "id"))
	return strings.TrimSpace(string(b))
}

// watchesDev reports whether wrangler.jsonc has wrangler watch .charter/built (its build's watch_dir), which is how a
// running wrangler dev is told of a new build. Without it the Worker is started again for each build.
func watchesDev(p *project.Project) bool {
	b, _ := os.ReadFile(filepath.Join(p.Root, "wrangler.jsonc"))
	return regexp.MustCompile(`"watch_dir"\s*:\s*"\.charter/built"`).Match(b)
}

// watch writes a part of the app as Go again (charter build -only <only>) whenever one of its files changes, for as
// long as dev runs. It looks ten times a second: a handful of files, on every system, with no watcher to set up. gsx dev
// sees the Go change and builds. When the part does not build, what is wrong is kept (.charter/dev/<only>.err) and
// the build is made to run, which then fails with it: so the mistake is in the browser's overlay, as a mistake in a
// gsx file is, until a save that works takes it away.
func watch(p *project.Project, what, only string, files func() []string) {
	stamp := func() string {
		var b strings.Builder
		for _, f := range files() {
			if info, err := os.Stat(f); err == nil {
				fmt.Fprintf(&b, "%s %d %d\n", f, info.Size(), info.ModTime().UnixNano())
			}
		}
		return b.String()
	}
	last := stamp()
	if last == "" {
		return
	}
	failed := filepath.Join(devDir(p), only+".err")
	for range time.Tick(100 * time.Millisecond) {
		now := stamp()
		if now == last {
			continue
		}
		last = now
		// As its own program: a file half-written by an editor fails this run, not dev.
		cmd := exec.Command(self(), "build", "-only", only)
		cmd.Dir = p.Root
		out, err := cmd.CombinedOutput()
		_, was := os.Stat(failed)
		if err != nil {
			fmt.Printf("charter dev: the %s did not build (fix it and save again):\n%s", what, out)
			os.WriteFile(failed, out, 0o644)
			poke(p)
			continue
		}
		fmt.Printf("charter dev: %s built\n", what)
		if was == nil {
			os.Remove(failed)
		}
		// What was written may be data alone (a page of the docs, a translation), which gsx dev does not watch:
		// it is told there is something to build. When Go changed too, it has seen that itself.
		poke(p)
	}
}

// poke makes gsx dev build: it watches the app's Go files, so one of them changes. poke.built.go is a file Go itself
// leaves out of every build (its build constraint), so the change costs no build of the Go: when only a page of the
// docs or a translation changed, the Wasm is as it was and the Worker is swapped with the new data alone. It is not
// in git, like every *.built.go.
func poke(p *project.Project) {
	os.WriteFile(filepath.Join(p.Root, "poke.built.go"), fmt.Appendf(nil, "//go:build ignore\n\n// Code generated by charter dev, which changes it to have gsx dev build. DO NOT EDIT.\n\npackage main\n\n// %d\n", time.Now().UnixNano()), 0o644)
}

// devFailures are what charter dev's watchers could not build, for charter build -dev to fail with.
func devFailures(p *project.Project) error {
	files, _ := filepath.Glob(filepath.Join(devDir(p), "*.err"))
	var all []string
	for _, f := range files {
		if b, err := os.ReadFile(f); err == nil {
			all = append(all, strings.TrimSpace(string(b)))
		}
	}
	if len(all) == 0 {
		return nil
	}
	return errors.New(strings.Join(all, "\n\n"))
}
