package previewbuild_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joeblew999/charter/go/lib/internal/previewbuild"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPackUnpack(t *testing.T) {
	src := writeTree(t, map[string]string{
		"build/app.wasm":    "wasm",
		"build/worker.mjs":  "js",
		"dist/site/a.css":   "css",
		"dist/site/x/b.js":  "b",
		"unrelated/file.go": "not packed",
	})
	if err := os.Chmod(filepath.Join(src, "build/worker.mjs"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := previewbuild.Pack(src, "abc", "build", "dist/site")
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	m, err := previewbuild.Unpack(bytes.NewReader(data), dst, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 4 {
		t.Errorf("manifest has %d files, want 4: %v", len(m.Files), m.Files)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "dist/site/x/b.js")); string(b) != "b" {
		t.Errorf("dist/site/x/b.js = %q", b)
	}
	if fi, err := os.Stat(filepath.Join(dst, "build/worker.mjs")); err != nil || fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("executable bit lost: %v %v", fi, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "unrelated/file.go")); err == nil {
		t.Error("unrelated file was packed")
	}

	if _, err := previewbuild.Unpack(bytes.NewReader(data), t.TempDir(), "other"); err == nil || !strings.Contains(err.Error(), "for commit abc") {
		t.Errorf("wrong commit accepted: %v", err)
	}
	if _, err := previewbuild.Unpack(strings.NewReader("not gzip"), t.TempDir(), "abc"); err == nil {
		t.Error("garbage accepted")
	}
	if _, err := previewbuild.Pack(src, "", "build"); err == nil {
		t.Error("empty commit accepted")
	}
	if _, err := previewbuild.Pack(src, "abc", "nope"); err == nil {
		t.Error("missing path accepted")
	}
}

// handPack builds a gzip tar from name → content, so tests can make archives Pack never would.
func handPack(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, b := range files {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(b)), Typeflag: tar.TypeReg})
		tw.Write(b)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestUnpackRejects(t *testing.T) {
	sum := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
	manifest := func(files map[string]string) []byte {
		b, _ := json.Marshal(previewbuild.Manifest{Commit: "abc", Files: files})
		return b
	}
	for name, files := range map[string]map[string][]byte{
		"changed file":      {"build/app.wasm": []byte("evil"), previewbuild.ManifestName: manifest(map[string]string{"build/app.wasm": sum("wasm")})},
		"missing file":      {previewbuild.ManifestName: manifest(map[string]string{"build/app.wasm": sum("wasm")})},
		"unlisted file":     {"build/app.wasm": []byte("wasm"), "x.js": []byte("x"), previewbuild.ManifestName: manifest(map[string]string{"build/app.wasm": sum("wasm")})},
		"no manifest":       {"build/app.wasm": []byte("wasm")},
		"path leaving root": {"../escape": []byte("x"), previewbuild.ManifestName: manifest(map[string]string{"../escape": sum("x")})},
	} {
		dir := t.TempDir()
		if _, err := previewbuild.Unpack(bytes.NewReader(handPack(t, files)), dir, "abc"); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if left, _ := os.ReadDir(dir); len(left) != 0 {
			t.Errorf("%s: a rejected build left files behind: %v", name, left)
		}
	}
	ok := map[string][]byte{"build/app.wasm": []byte("wasm"), previewbuild.ManifestName: manifest(map[string]string{"build/app.wasm": sum("wasm")})}
	if _, err := previewbuild.Unpack(bytes.NewReader(handPack(t, ok)), t.TempDir(), "abc"); err != nil {
		t.Errorf("valid hand-made build rejected: %v", err)
	}
}

func TestSlug(t *testing.T) {
	for _, tc := range []struct{ branch, want string }{
		{"faster-workers-builds", "faster-workers-builds"},
		{"Feature/Login_Page", "feature-login-page"},
		{"--x--", "x"},
		{"2026-fix", "b-2026-fix"},
		{"///", "branch"},
	} {
		if got := previewbuild.Slug(tc.branch, "app"); got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.branch, got, tc.want)
		}
	}
	long := strings.Repeat("very-long-branch-name-", 6)
	a, b := previewbuild.Slug(long+"a", "app"), previewbuild.Slug(long+"b", "app")
	if a == b {
		t.Errorf("long branches collide: %q", a)
	}
	for _, s := range []string{a, b} {
		if l := len(s) + len("-app"); l > 63 {
			t.Errorf("%q: label %d > 63", s, l)
		}
		if strings.HasSuffix(s, "-") || strings.Contains(s, "--") {
			t.Errorf("%q: bad hyphens", s)
		}
	}
	if got := previewbuild.URL("pr-7", "app", "acme"); got != "https://pr-7-app.acme.workers.dev" {
		t.Errorf("URL = %s", got)
	}
}

// fakeGitHub implements releases, assets, downloads, pulls and comments for repo o/r.
type fakeGitHub struct {
	mu       sync.Mutex
	release  int64
	assets   map[int64]map[string]any // id → asset
	blobs    map[string][]byte        // asset name → content
	nextID   int64
	prs      map[string]int // branch → PR number
	comments map[int][]map[string]any
	delay    int // downloads 404 this many times first
	calls    []string
}

func newFakeGitHub(t *testing.T) (*fakeGitHub, *previewbuild.GitHub) {
	f := &fakeGitHub{assets: map[int64]map[string]any{}, blobs: map[string][]byte{}, prs: map[string]int{}, comments: map[int][]map[string]any{}, nextID: 100}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, &previewbuild.GitHub{Repo: "o/r", Token: "tok", API: srv.URL, Uploads: srv.URL + "/up", Downloads: srv.URL + "/dl"}
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	reply := func(status int, v any) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	write := strings.HasPrefix(r.URL.Path, "/repos/") || strings.HasPrefix(r.URL.Path, "/up/")
	if write && r.Method != "GET" && r.Header.Get("Authorization") != "Bearer tok" {
		reply(401, map[string]string{"message": "bad token"})
		return
	}
	p := r.URL.Path
	switch {
	case r.Method == "GET" && p == "/repos/o/r/releases/tags/preview-builds":
		if f.release == 0 {
			reply(404, map[string]string{"message": "Not Found"})
			return
		}
		reply(200, f.rel())
	case r.Method == "POST" && p == "/repos/o/r/releases":
		f.release = 7
		reply(201, f.rel())
	case r.Method == "POST" && p == fmt.Sprintf("/up/repos/o/r/releases/%d/assets", f.release):
		name := r.URL.Query().Get("name")
		b, _ := io.ReadAll(r.Body)
		f.nextID++
		f.assets[f.nextID] = map[string]any{"id": f.nextID, "name": name, "created_at": time.Unix(1_700_000_000+f.nextID, 0).UTC()}
		f.blobs[name] = b
		reply(201, f.assets[f.nextID])
	case r.Method == "DELETE" && strings.HasPrefix(p, "/repos/o/r/releases/assets/"):
		var id int64
		fmt.Sscanf(strings.TrimPrefix(p, "/repos/o/r/releases/assets/"), "%d", &id)
		a, ok := f.assets[id]
		if !ok {
			reply(404, nil)
			return
		}
		delete(f.blobs, a["name"].(string))
		delete(f.assets, id)
		w.WriteHeader(204)
	case r.Method == "GET" && strings.HasPrefix(p, "/dl/o/r/releases/download/preview-builds/"):
		if f.delay > 0 {
			f.delay--
			w.WriteHeader(404)
			return
		}
		b, ok := f.blobs[strings.TrimPrefix(p, "/dl/o/r/releases/download/preview-builds/")]
		if !ok {
			w.WriteHeader(404)
			return
		}
		w.Write(b)
	case r.Method == "GET" && p == "/repos/o/r/pulls":
		var out []map[string]int
		if n, ok := f.prs[strings.TrimPrefix(r.URL.Query().Get("head"), "o:")]; ok && r.URL.Query().Get("state") == "open" {
			out = append(out, map[string]int{"number": n})
		}
		reply(200, out)
	case strings.HasPrefix(p, "/repos/o/r/issues/") && strings.HasSuffix(p, "/comments"):
		var n int
		fmt.Sscanf(strings.TrimPrefix(p, "/repos/o/r/issues/"), "%d", &n)
		if r.Method == "GET" {
			reply(200, f.comments[n])
			return
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		f.nextID++
		f.comments[n] = append(f.comments[n], map[string]any{"id": f.nextID, "body": body["body"]})
		reply(201, nil)
	case r.Method == "PATCH" && strings.HasPrefix(p, "/repos/o/r/issues/comments/"):
		var id int64
		fmt.Sscanf(strings.TrimPrefix(p, "/repos/o/r/issues/comments/"), "%d", &id)
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		for _, cs := range f.comments {
			for _, c := range cs {
				if c["id"].(int64) == id {
					c["body"] = body["body"]
				}
			}
		}
		reply(200, nil)
	default:
		reply(404, map[string]string{"message": r.Method + " " + p})
	}
}

func (f *fakeGitHub) rel() map[string]any {
	var as []map[string]any
	for _, a := range f.assets {
		as = append(as, a)
	}
	return map[string]any{"id": f.release, "assets": as}
}

func TestPublishFetch(t *testing.T) {
	f, gh := newFakeGitHub(t)
	ctx := context.Background()

	if err := gh.Publish(ctx, "preview-builds", "a.tar.gz", []byte("one"), 3); err != nil {
		t.Fatal(err)
	}
	if f.release == 0 {
		t.Fatal("release not created")
	}
	if err := gh.Publish(ctx, "preview-builds", "a.tar.gz", []byte("two"), 3); err != nil { // replace
		t.Fatal(err)
	}
	if len(f.assets) != 1 || string(f.blobs["a.tar.gz"]) != "two" {
		t.Fatalf("replace: assets %v blobs %v", f.assets, f.blobs)
	}
	for _, n := range []string{"b", "c", "d", "e"} {
		if err := gh.Publish(ctx, "preview-builds", n+".tar.gz", []byte(n), 3); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.assets) != 3 {
		t.Errorf("kept %d assets, want 3: %v", len(f.blobs), f.blobs)
	}
	for _, n := range []string{"c", "d", "e"} {
		if _, ok := f.blobs[n+".tar.gz"]; !ok {
			t.Errorf("newest %s.tar.gz pruned", n)
		}
	}

	f.delay = 2
	b, ok, err := gh.Fetch(ctx, "preview-builds", "e.tar.gz", time.Second, time.Millisecond)
	if err != nil || !ok || string(b) != "e" {
		t.Fatalf("fetch after waiting: %q %v %v", b, ok, err)
	}
	_, ok, err = gh.Fetch(ctx, "preview-builds", "missing.tar.gz", 20*time.Millisecond, 5*time.Millisecond)
	if err != nil || ok {
		t.Errorf("missing asset: ok %v err %v", ok, err)
	}

	bad := *gh
	bad.Token = "wrong"
	if err := bad.Publish(ctx, "preview-builds", "x.tar.gz", []byte("x"), 3); err == nil {
		t.Error("publish with a bad token succeeded")
	}
}

func TestCommentPR(t *testing.T) {
	f, gh := newFakeGitHub(t)
	ctx := context.Background()
	if ok, err := gh.CommentPR(ctx, "no-pr", "<!-- p -->", "x"); err != nil || ok {
		t.Errorf("branch without PR: %v %v", ok, err)
	}
	f.prs["feat"] = 9
	for _, body := range []string{"first", "second"} {
		if ok, err := gh.CommentPR(ctx, "feat", "<!-- p -->", body); err != nil || !ok {
			t.Fatalf("comment %s: %v %v", body, ok, err)
		}
	}
	if cs := f.comments[9]; len(cs) != 1 || cs[0]["body"] != "<!-- p -->\nsecond" {
		t.Errorf("want one edited comment, got %v", cs)
	}
}
