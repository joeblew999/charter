//go:build !(js && wasm)

package bindings_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joeblew999/charter/bindings"
)

// TestOffWorkers: off Workers, a rate limit counts as wrangler.jsonc says, per key; a binding it doesn't have is
// allowed and reported; Static Assets are the folder from disk; a variable is the environment's.
func TestOffWorkers(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"wrangler.jsonc": `{"name": "t", "ratelimits": [{"name": "WRITES", "namespace_id": "1", "simple": {"limit": 2, "period": 60}}]}`,
		"site/a.txt":     "asset\n",
	} {
		os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755)
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}
	sub := filepath.Join(dir, "app")
	os.MkdirAll(sub, 0o755)
	t.Chdir(sub) // a test runs in its package's folder, below wrangler.jsonc

	for i, want := range []bool{true, true, false} {
		if ok, err := bindings.Allow("WRITES", "me"); ok != want || err != nil {
			t.Errorf("call %d: %v, %v; want %v", i+1, ok, err, want)
		}
	}
	if ok, err := bindings.Allow("WRITES", "you"); !ok || err != nil {
		t.Errorf("another key: %v, %v", ok, err)
	}
	if ok, err := bindings.Allow("READS", "me"); !ok || !errors.Is(err, bindings.ErrNoBinding) {
		t.Errorf("no such binding: %v, %v", ok, err)
	}

	rec := httptest.NewRecorder()
	bindings.StaticAssets("site").ServeHTTP(rec, httptest.NewRequest("GET", "/a.txt", nil))
	if rec.Code != 200 || rec.Body.String() != "asset\n" {
		t.Errorf("asset: %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	bindings.StaticAssets("site").ServeHTTP(rec, httptest.NewRequest("POST", "/a.txt", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST an asset: %d", rec.Code)
	}

	t.Setenv("APP_ENV", "test")
	if bindings.Var("APP_ENV") != "test" || bindings.OnWorkers {
		t.Errorf("Var = %q, OnWorkers = %v", bindings.Var("APP_ENV"), bindings.OnWorkers)
	}
	r := httptest.NewRequest("GET", "/", nil)
	if bindings.ClientKey(r) != "local" {
		t.Errorf("ClientKey without CF-Connecting-IP = %q", bindings.ClientKey(r))
	}
	r.Header.Set("CF-Connecting-IP", "203.0.113.7")
	if bindings.ClientKey(r) != "203.0.113.7" {
		t.Errorf("ClientKey = %q", bindings.ClientKey(r))
	}
}

// TestHTTPClient: off Workers the client reaches a server, and each call gives a client of the caller's own, so a
// Timeout set on one is not another's.
func TestHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))
	defer server.Close()
	client := bindings.HTTPClient()
	client.Timeout = time.Second
	res, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("status %d", res.StatusCode)
	}
	if other := bindings.HTTPClient(); other == client || other.Timeout != 0 {
		t.Errorf("a second client shares the first's settings: timeout %v", other.Timeout)
	}
}

// TestVarOffWorkers: off Workers a variable is what wrangler dev would give it, .dev.vars over wrangler.jsonc's vars,
// whatever the process's environment says; a name neither has is the environment's; and an edit to wrangler.jsonc is
// the next call's.
func TestVarOffWorkers(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("wrangler.jsonc", `{"name": "t", "vars": {"APP_NAME": "shop", "MAX": 3}}`)
	write(".dev.vars", "# local only\nTOKEN=local-token\n")
	t.Chdir(dir)
	t.Setenv("APP_NAME", "from the shell")
	t.Setenv("ONLY_ENV", "set by a test")
	// A test's own value comes first: that is how it gives a variable one.
	if got := bindings.Var("APP_NAME"); got != "from the shell" {
		t.Errorf("under go test, Var(APP_NAME) = %q, want the test's own value", got)
	}
	// Outside a test, a developer's shell never changes what the app shows.
	defer bindings.NotUnderTest()()
	for name, want := range map[string]string{"APP_NAME": "shop", "MAX": "3", "TOKEN": "local-token", "ONLY_ENV": "set by a test", "NOPE": ""} {
		if got := bindings.Var(name); got != want {
			t.Errorf("Var(%s) = %q, want %q", name, got, want)
		}
	}
	write("wrangler.jsonc", `{"name": "t", "vars": {"APP_NAME": "shop, renamed"}}`)
	os.Chtimes(filepath.Join(dir, "wrangler.jsonc"), time.Now().Add(time.Second), time.Now().Add(time.Second))
	if got := bindings.Var("APP_NAME"); got != "shop, renamed" {
		t.Errorf("after an edit, Var(APP_NAME) = %q", got)
	}
}
