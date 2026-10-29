package build

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestShared: a build the repository's cache has is fetched by its key; one it hasn't is left to compile; and a
// built Wasm is stored there under its key, but only with a token.
func TestShared(t *testing.T) {
	var mu sync.Mutex
	assets := map[string]string{"wasm-k1": "WASM1"}
	uploaded := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/releases/download/charter-cache/"):
			name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			if body, ok := assets[name]; ok {
				io.WriteString(w, body)
				return
			}
			http.NotFound(w, r)
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/releases/tags/charter-cache"):
			io.WriteString(w, `{"id":1,"assets":[]}`)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/releases/1/assets"):
			b, _ := io.ReadAll(r.Body)
			uploaded = r.URL.Query().Get("name") + "=" + string(b)
			io.WriteString(w, `{"id":2,"name":"x"}`)
		default:
			io.WriteString(w, `[]`)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("CHARTER_CACHE_REPO", "me/app")
	t.Setenv("GITHUB_TOKEN", "")
	sharedURL = srv.URL
	defer func() { sharedURL = "" }()

	fetchShared("k1", filepath.Join(dir, "k1.wasm"))
	if b, _ := os.ReadFile(filepath.Join(dir, "k1.wasm")); string(b) != "WASM1" {
		t.Errorf("a cached build wasn't fetched: %q", b)
	}
	fetchShared("k2", filepath.Join(dir, "k2.wasm"))
	if _, err := os.Stat(filepath.Join(dir, "k2.wasm")); err == nil {
		t.Error("a build the cache hasn't was made up")
	}
	os.WriteFile(filepath.Join(dir, "app.wasm"), []byte("WASM2"), 0o644)
	storeShared("k2", filepath.Join(dir, "app.wasm"))
	if uploaded != "" {
		t.Errorf("stored without a token: %q", uploaded)
	}
	t.Setenv("GITHUB_TOKEN", "t")
	storeShared("k2", filepath.Join(dir, "app.wasm"))
	if uploaded != "wasm-k2=WASM2" {
		t.Errorf("stored %q, want wasm-k2=WASM2", uploaded)
	}
}
