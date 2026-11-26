// Package apptest is for an app's tests of its Worker (the app's test/ folder), which go tool charter test runs
// against the Worker on wrangler dev, and go tool charter test --deployed against the deployed one. The same tests
// ask both; a test that only one can answer says so with Local.
//
//	func TestHome(t *testing.T) {
//		if !strings.Contains(apptest.Get(t, "/"), "<h1>") { … }
//	}
//
// test/dev holds tests for a local Worker only, test/deployed for a deployed one only.
package apptest

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Base is the Worker's address (charter's BASE), without a trailing slash. A test run without one (go test ./...,
// which asks no Worker) is skipped.
func Base(t testing.TB) string {
	t.Helper()
	base := strings.TrimRight(os.Getenv("BASE"), "/")
	if base == "" {
		t.Skip("no BASE, the Worker's address: go tool charter test")
	}
	return base
}

// Local says the Worker is wrangler dev on this machine (charter's TARGET), which can do what a deployed one cannot:
// have a Cron Trigger fired by hand, be handed an email. It also answers at once where Cloudflare takes its time.
func Local() bool { return os.Getenv("TARGET") != "deployed" }

// Get is the body of a GET of path, which must answer 200.
func Get(t testing.TB, path string, headers ...string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, Base(t)+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: HTTP %d\n%s", path, res.StatusCode, b)
	}
	return string(b)
}

// Secret is a secret of the Worker for a test that needs one (a token): the environment's, or for a local Worker
// the throwaway value in the app's .dev.vars. A deployed Worker's secret that the environment lacks skips the test.
func Secret(t testing.TB, name string) string {
	t.Helper()
	if v := os.Getenv(name); v != "" {
		return v
	}
	if Local() {
		dir, _ := os.Getwd()
		for ; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			b, err := os.ReadFile(filepath.Join(dir, ".dev.vars"))
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(b), "\n") {
				if key, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok && key == name {
					return strings.Trim(value, `"'`)
				}
			}
		}
	}
	t.Skipf("no %s: the deployed Worker's secret, in the environment", name)
	return ""
}

// Events opens a server-sent event stream at path and returns a function that gives its next event (its lines,
// without comments), failing the test when none comes in time. The stream closes with the test.
func Events(t testing.TB, path string, header http.Header) (next func() string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, Base(t)+path, nil)
	for name, values := range header {
		req.Header[name] = values
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("GET %s: HTTP %d %s, want an event stream", path, res.StatusCode, res.Header.Get("Content-Type"))
	}
	events := make(chan string, 16)
	go func() {
		lines := bufio.NewScanner(res.Body)
		lines.Buffer(nil, 1<<20)
		var event strings.Builder
		for lines.Scan() {
			if line := lines.Text(); line != "" {
				if !strings.HasPrefix(line, ":") {
					event.WriteString(line + "\n")
				}
				continue
			}
			if event.Len() > 0 {
				events <- event.String()
				event.Reset()
			}
		}
	}()
	return func() string {
		t.Helper()
		select {
		case event := <-events:
			return event
		case <-time.After(15 * time.Second):
			t.Fatalf("GET %s: no event in 15 s", path)
			return ""
		}
	}
}
