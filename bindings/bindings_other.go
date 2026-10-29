//go:build !(js && wasm)

package bindings

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/joeblew999/charter/project"
	"github.com/joeblew999/charter/web"
)

// OnWorkers reports whether the code runs as a Worker.
const OnWorkers = false

// Var is the process's environment variable called name, standing in for the Worker's variable.
func Var(name string) string { return os.Getenv(name) }

// Allow counts key's calls to the limit charter.toml's [[rate_limits]] names, in memory: a fixed window of its period,
// as the binding counts.
func Allow(name, key string) (bool, error) {
	p, err := app()
	if err != nil {
		return true, fmt.Errorf("%w: %s (%v)", ErrNoBinding, name, err)
	}
	limit, ok := p.RateLimit(name)
	if !ok {
		return true, fmt.Errorf("%w: %s (charter.toml has no such [[rate_limits]])", ErrNoBinding, name)
	}
	limits.mu.Lock()
	defer limits.mu.Unlock()
	now := time.Now()
	w := limits.windows[name+"\x00"+key]
	if w == nil || now.Sub(w.start) >= time.Duration(limit.Period)*time.Second {
		limits.windows[name+"\x00"+key] = &window{start: now, count: 1}
		return true, nil
	}
	w.count++
	return w.count <= limit.Limit, nil
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

// app is charter.toml, found from the working directory (a test runs in its package's folder) and read once.
var app = sync.OnceValues(func() (*project.Project, error) { return project.Load(".") })
