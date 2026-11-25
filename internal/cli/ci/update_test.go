package ci

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeblew999/charter/project"
)

// TestUpdate: the weekly job. An update that changes nothing opens nothing; one that changes a file commits it to
// the update branch, pushes it and opens a pull request that links the run; a second finds that pull request and
// opens no other; an update that fails pushes nothing.
func TestUpdate(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	os.WriteFile(filepath.Join(root, "pins.txt"), []byte("1\n"), 0o644)
	git("init", "-q", "-b", "main")
	git("add", "-A")
	git("commit", "-q", "-m", "first")

	var pushed, opened []string
	open := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/repos/o/r/pulls":
			if r.URL.Query().Get("head") != "o:"+UpdateBranch {
				t.Errorf("asked for the pull requests of %q", r.URL.Query().Get("head"))
			}
			if open {
				io.WriteString(w, `[{"html_url":"https://github.com/o/r/pull/7"}]`)
			} else {
				io.WriteString(w, `[]`)
			}
		case r.Method == "POST" && r.URL.Path == "/repos/o/r/pulls":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			opened = append(opened, body["head"]+"→"+body["base"]+": "+body["body"])
			open = true
			w.WriteHeader(201)
			io.WriteString(w, `{"html_url":"https://github.com/o/r/pull/7"}`)
		default:
			http.Error(w, "no", 404)
		}
	}))
	defer srv.Close()
	keep := pushBranch
	defer func() { pushBranch = keep }()
	pushBranch = func(gh github, root, branch string) error {
		pushed = append(pushed, branch+" "+strings.TrimSpace(git("log", "--format=%s", "-1", branch)))
		return nil
	}
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	t.Setenv("GITHUB_SERVER_URL", "https://github.com")
	t.Setenv("GITHUB_REPOSITORY", "o/r")
	t.Setenv("GITHUB_RUN_ID", "42")
	gh := github{repo: "o/r", token: "tok", api: srv.URL}
	p := func(command string) *project.Project {
		return &project.Project{Root: root, CI: project.CI{Update: command}}
	}

	if code := update(p("true"), gh); code != 0 || len(pushed) != 0 || len(opened) != 0 {
		t.Fatalf("an update that changes nothing: exit %d, pushed %v, opened %v", code, pushed, opened)
	}
	if code := update(p("echo 2 > '"+filepath.Join(root, "pins.txt")+"' && exit 1"), gh); code != 1 || len(pushed) != 0 {
		t.Fatalf("an update that fails: exit %d, pushed %v", code, pushed)
	}
	git("checkout", "-q", "--", "pins.txt")
	if code := update(p("echo 2 > '"+filepath.Join(root, "pins.txt")+"'"), gh); code != 0 {
		t.Fatalf("an update that moves a pin: exit %d", code)
	}
	if len(pushed) != 1 || pushed[0] != UpdateBranch+" Update pins" {
		t.Errorf("pushed %v", pushed)
	}
	if len(opened) != 1 || !strings.HasPrefix(opened[0], UpdateBranch+"→main: ") || !strings.Contains(opened[0], "https://github.com/o/r/actions/runs/42") || !strings.Contains(opened[0], "pins.txt") {
		t.Errorf("opened %v", opened)
	}
	// The next week: the branch moves again from main, and the open pull request is kept.
	git("checkout", "-q", "main")
	if code := update(p("echo 3 > '"+filepath.Join(root, "pins.txt")+"'"), gh); code != 0 || len(pushed) != 2 || len(opened) != 1 {
		t.Errorf("a second update: exit %d, pushed %v, opened %d pull requests", code, pushed, len(opened))
	}
}
