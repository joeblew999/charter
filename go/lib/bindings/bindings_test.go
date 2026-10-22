//go:build !(js && wasm)

package bindings_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/joeblew999/charter/go/lib/bindings"
)

// TestOffWorkers: off Workers, a rate limit counts as charter.toml says, per key; a binding it doesn't have is
// allowed and reported; Static Assets are the folder from disk; a variable is the environment's.
func TestOffWorkers(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"charter.toml": "name = \"t\"\ncompatibility_date = \"2026-09-11\"\n[[rate_limits]]\nbinding = \"WRITES\"\nnamespace_id = 1\nlimit = 2\nperiod = 60\n",
		"site/a.txt":   "asset\n",
	} {
		os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755)
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}
	sub := filepath.Join(dir, "app")
	os.MkdirAll(sub, 0o755)
	t.Chdir(sub) // a test runs in its package's folder, below charter.toml

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
