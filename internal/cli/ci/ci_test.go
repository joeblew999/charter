package ci

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestRelease: a commit that isn't on main isn't released; a release branch is tagged, the hook runs, production
// moves and the branch goes.
func TestRelease(t *testing.T) {
	var calls []string
	status := "ahead"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+strings.TrimPrefix(r.URL.Path, "/repos/me/app"))
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]string{"status": status})
		}
	}))
	defer srv.Close()
	gh := github{repo: "me/app", token: "t", api: srv.URL}
	var moved string
	pushProduction = func(_ github, sha string) error { moved = sha; return nil }
	hooked := filepath.Join(t.TempDir(), "hooked")

	status = "diverged"
	if err := release(gh, "v1.2", "abc1234", "release/v1.2", "branch", ""); err == nil || moved != "" {
		t.Fatalf("a commit off main: %v, production moved to %q", err, moved)
	}
	calls, status = nil, "ahead"
	if err := release(gh, "v1.2", "abc1234", "release/v1.2", "branch", "touch "+hooked); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /compare/abc1234...main", "POST /git/refs", "DELETE /git/refs/heads/release/v1.2"}
	if !slices.Equal(calls, want) || moved != "abc1234" {
		t.Errorf("calls %v, want %v; production → %q", calls, want, moved)
	}
	if _, err := os.Stat(hooked); err != nil {
		t.Error("the hook didn't run")
	}
	calls = nil
	if err := release(gh, "v1.3", "def5678", "v1.3", "tag", ""); err != nil || !slices.Equal(calls, []string{"GET /compare/def5678...main"}) {
		t.Errorf("a pushed tag: %v, calls %v", err, calls)
	}
}

// TestReleaseArgs: what ci run passes, ci release reads (v1.8's release stopped at the usage line: the flags were
// parsed before the step's name).
func TestReleaseArgs(t *testing.T) {
	for _, hook := range []string{"", `ci/module-tags.sh "$GITHUB_SHA"`} {
		step, got, rest, err := parse(releaseArgs(hook, "v1.8"))
		if err != nil || step != "release" || got != hook || !slices.Equal(rest, []string{"v1.8"}) {
			t.Errorf("hook %q: step %q, hook %q, rest %v, %v", hook, step, got, rest, err)
		}
	}
}

func TestReleaseOf(t *testing.T) {
	for _, tc := range []struct{ refType, ref, want string }{
		{"tag", "v1.8", "v1.8"}, {"branch", "release/v1.8", "v1.8"}, {"branch", "main", ""}, {"tag", "intl/v0.7.0", ""}, {"branch", "v1.8", ""},
	} {
		if got := releaseOf(tc.refType, tc.ref); got != tc.want {
			t.Errorf("releaseOf(%s, %s) = %q, want %q", tc.refType, tc.ref, got, tc.want)
		}
	}
}
