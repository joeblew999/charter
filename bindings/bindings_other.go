//go:build !(js && wasm)

package bindings

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/web"
)

// OnWorkers reports whether the code runs as a Worker.
const OnWorkers = false

// Var stands in for the Worker's variable called name, with the value wrangler dev would give it: .dev.vars's, else
// wrangler.jsonc's top-level vars. A name neither has (one only an environment of wrangler.jsonc sets, such as
// production's) is the process's environment variable, which is how a test gives it a value.
func Var(name string) string {
	if p, err := app(); err == nil {
		if v, ok := p.Var(name); ok {
			return v
		}
	}
	return os.Getenv(name)
}

// HTTPClient is a client for the requests the app makes, standing in for the Worker's fetch: net/http's own
// transport. Each call returns a new client, the caller's to set (its Timeout).
func HTTPClient() *http.Client { return &http.Client{} }

// Allow counts key's calls to the limit wrangler.jsonc's ratelimits names, in memory: a fixed window of its period,
// as the binding counts.
func Allow(name, key string) (bool, error) {
	p, err := app()
	if err != nil {
		return true, fmt.Errorf("%w: %s (%v)", ErrNoBinding, name, err)
	}
	limit, ok := p.RateLimit(name)
	if !ok {
		return true, fmt.Errorf("%w: %s (wrangler.jsonc's ratelimits has none of that name)", ErrNoBinding, name)
	}
	limits.mu.Lock()
	defer limits.mu.Unlock()
	now := time.Now()
	w := limits.windows[name+"\x00"+key]
	if w == nil || now.Sub(w.start) >= time.Duration(limit.Simple.Period)*time.Second {
		limits.windows[name+"\x00"+key] = &window{start: now, count: 1}
		return true, nil
	}
	w.count++
	return w.count <= limit.Simple.Limit, nil
}

type window struct {
	start time.Time
	count int
}

var limits = struct {
	mu      sync.Mutex
	windows map[string]*window
}{windows: map[string]*window{}}

// StaticAssets serves dir (relative to charter.toml's folder) from disk, GET and HEAD only, standing in for Static
// Assets.
func StaticAssets(dir string) http.Handler {
	if p, err := app(); err == nil && !filepath.IsAbs(dir) {
		dir = filepath.Join(p.Root, dir)
	}
	return web.GetOnly(http.FileServer(http.Dir(dir)))
}

// app is the app's settings (wrangler.jsonc, charter.toml), found from the working directory (a test runs in its
// package's folder). They are read again when wrangler.jsonc has changed, so a variable or a rate limit edited while
// the dev loop runs is the next request's, with no restart.
func app() (*project.Project, error) {
	settings.mu.Lock()
	defer settings.mu.Unlock()
	wd, _ := os.Getwd()
	if settings.p != nil && wd == settings.wd {
		if info, err := os.Stat(filepath.Join(settings.p.Root, project.Wrangler)); err == nil && info.ModTime().Equal(settings.at) {
			return settings.p, nil
		}
	}
	p, err := project.Load(".")
	if err != nil {
		return nil, err
	}
	settings.p, settings.wd = p, wd
	if info, err := os.Stat(filepath.Join(p.Root, project.Wrangler)); err == nil {
		settings.at = info.ModTime()
	}
	return p, nil
}

var settings struct {
	mu sync.Mutex
	p  *project.Project
	wd string // where it was found from: a test in another folder is another app
	at time.Time
}
