package command

import (
	"errors"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	intlconfig "github.com/joeblew999/charter/intl/config"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Dev runs charter dev: the loop a developer works in, on the runtime the app ships on.
//
// With gsx (a gsx.toml) it is gsx's own loop, go tool gsx dev, with its two documented hooks: the build is charter
// build -dev (the Wasm, about half a second) and the server is wrangler dev (about a second to start when the build
// is there). So a save regenerates, rebuilds, restarts the Worker and reloads the browser through Vite, and what is
// on screen is workers-go on the Workers runtime with wrangler.jsonc's bindings, as in production. Without gsx it is
// wrangler dev alone.
//
// Everything else a developer runs meanwhile, in another terminal, reaches the open page by itself: gsx dev watches
// the Go and gsx files (gsxui add), Vite the browser's files (npm install), and charter the languages: a change to a
// catalog or to charter.toml (charter add locale:<id>, charter run translate, an edit by hand) compiles the
// messages again, which gsx dev then picks up.
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
	check(generate(p)) // the first build needs the messages and Vite's manifest
	check(migrate(p, map[string]string{"WRANGLER_SEND_METRICS": "false"}, "--local"))
	if hasIntl(p) {
		go watchLanguages(p)
	}
	// As gsx's flags take them: a command line, split on spaces. go tool charter is this charter, by the app's go.mod.
	gsx := []string{"tool", "gsx", "dev", "--build", "go tool charter build -dev", "--run", "go tool charter dev -worker"}
	check(passthrough(p, "go", append(gsx, args...)...))
}

// devWorker is charter dev -worker, the server gsx dev starts after each build and stops before the next: wrangler
// dev on the port gsx chose (GO_PORT), told where Vite's dev server is (VITE_DEV_URL, which app.App reads as a
// variable of the Worker) and that the build is done.
func devWorker() {
	p := load()
	port := os.Getenv("GO_PORT")
	if port == "" {
		check(errors.New("charter dev -worker is what charter dev has gsx run: it needs GO_PORT, which gsx dev sets"))
	}
	inspector, err := run.FreePort()
	check(err)
	bin, err := npm(p, "wrangler")
	check(err)
	// --test-scheduled: a Cron Trigger can be fired by hand, GET /__scheduled?cron=*+*+*+*+*
	args := []string{"dev", "--port", port, "--inspector-port", strconv.Itoa(inspector), "--show-interactive-dev-session=false", "--test-scheduled"}
	if vite := os.Getenv("VITE_DEV_URL"); vite != "" {
		args = append(args, "--var", "VITE_DEV_URL:"+vite)
	}
	check(run.Foreground(p.Root, map[string]string{devBuilt: "1", "WRANGLER_SEND_METRICS": "false"}, bin, args...))
}

// watchLanguages compiles the messages again whenever a catalog or charter.toml changes, for as long as dev runs. It
// looks twice a second: a handful of files, on every system, with no watcher to set up.
func watchLanguages(p *project.Project) {
	c, err := intlconfig.Find(p.Root)
	if err != nil {
		return
	}
	stamp := func() string {
		files, _ := filepath.Glob(filepath.Join(c.CatalogDir(), "*.toml"))
		files = append(files, filepath.Join(p.Root, project.File))
		var b strings.Builder
		for _, f := range files {
			if info, err := os.Stat(f); err == nil {
				fmt.Fprintf(&b, "%s %d %d\n", f, info.Size(), info.ModTime().UnixNano())
			}
		}
		return b.String()
	}
	last := stamp()
	for range time.Tick(500 * time.Millisecond) {
		now := stamp()
		if now == last {
			continue
		}
		last = now
		// As its own program: a catalog half-written by an editor or a translator fails this run, not dev.
		cmd := exec.Command(self(), "build", "-only", "messages")
		cmd.Dir = p.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("charter dev: the languages did not compile (fix the catalog and save again):\n%s", out)
			continue
		}
		fmt.Println("charter dev: languages compiled")
	}
}
