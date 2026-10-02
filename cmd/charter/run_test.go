package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestWords(t *testing.T) {
	for line, want := range map[string][]string{
		"go run .":                               {"go", "run", "."},
		"  node  test/live-test.mjs\thttp://x  ": {"node", "test/live-test.mjs", "http://x"},
		`go test -run "A B" ./...`:               {"go", "test", "-run", "A B", "./..."},
		`node -e 'console.log("a b")' ""`:        {"node", "-e", `console.log("a b")`, ""},
		`a"b c"d`:                                {"ab cd"},
		"PORT=1 $HOME && x":                      {"PORT=1", "$HOME", "&&", "x"}, // words, not a shell line
	} {
		got, err := words(line)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("words(%q) = %q (%v), want %q", line, got, err, want)
		}
	}
	for _, line := range []string{"", "   ", `node "test`} {
		if got, err := words(line); err == nil {
			t.Errorf("words(%q) = %q, want an error", line, got)
		}
	}
}

// A program that an npm package of the project installed is the project's, before one on the path.
func TestProgramPrefersTheProjectsNpmInstall(t *testing.T) {
	t.Chdir(t.TempDir())
	if path, err := program("go"); err != nil || !filepath.IsAbs(path) || strings.Contains(path, "node_modules") {
		t.Errorf("program(go) = %q (%v), want the go on the path", path, err)
	}
	if _, err := program("no-such-program"); err == nil || !strings.Contains(err.Error(), "on the path") {
		t.Errorf("program(no-such-program): %v, want an error", err)
	}
	// The lockfile says the project installs a go of its own: not installed yet is an error, and the
	// one on the path is not taken in its place.
	if err := write("package-lock.json", `{"packages": {"": {}, "node_modules/go": {"bin": {"go": "bin/go.js"}}, "node_modules/a/node_modules/b": {"bin": {"node": "n.js"}}}}`); err != nil {
		t.Fatal(err)
	}
	if _, err := program("go"); err == nil || !strings.Contains(err.Error(), "mise run setup") {
		t.Errorf("program(go) before npm installed it: %v, want an error that names mise run setup", err)
	}
	if path, err := program("node"); err != nil || strings.Contains(path, "node_modules") {
		t.Errorf("program(node) = %q (%v), want the node on the path: only a package inside another installs one", path, err)
	}
	shim := filepath.Join("node_modules", ".bin", npmShim("go"))
	if err := write(shim, ""); err != nil {
		t.Fatal(err)
	}
	if path, err := program("go"); err != nil || !filepath.IsAbs(path) || !strings.HasSuffix(path, shim) {
		t.Errorf("program(go) = %q (%v), want %s", path, err, shim)
	}
}

// A variable that is already set is replaced, not set a second time: a program may read the first.
func TestEnvironSetsAVariableOnce(t *testing.T) {
	t.Setenv("PORT", "5173")
	var ports []string
	for _, variable := range environ([]string{"PORT=1", "PORT=2", "OTHER=x"}) {
		if strings.HasPrefix(variable, "PORT=") {
			ports = append(ports, variable)
		}
	}
	if !slices.Equal(ports, []string{"PORT=2"}) {
		t.Errorf("PORT is %v, want only the last value given", ports)
	}
}

func TestLint(t *testing.T) {
	t.Chdir(t.TempDir())
	for file, content := range map[string]string{
		"go.mod":      "module example.com/lint\n\ngo 1.24\n",
		"main.go":     "package main\n\nfunc main() {}\n",
		"api/api.go":  "package api\n\nfunc Hello() string { return \"hello\" }\n",
		"skip/bad.go": "package skip\nfunc  Bad( ) {}\n",
	} {
		if err := write(file, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := lint([]string{"-vet", "./api,.", "-wasm", ".", "*.go", "api"}); err != nil {
		t.Errorf("lint of formatted code: %v", err)
	}
	if err := lint([]string{"-vet", ".", "*.go", "skip"}); err == nil || !strings.Contains(filepath.ToSlash(err.Error()), "skip/bad.go") {
		t.Errorf("lint of skip/bad.go: %v, want an error that names it", err)
	}
	if err := lint([]string{"*.txt"}); err == nil || !strings.Contains(err.Error(), "no file matches") {
		t.Errorf("lint *.txt: %v, want an error", err)
	}
}

// with-server's server is this test program again (TestMain below): it starts a second copy of
// itself, which is the one that listens. Stopping the server must stop that one too, as it must
// stop the program under `go run` and the workerd under `cf dev`.
func TestStoppingAServerStopsWhatItStarted(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	url := "http://localhost:" + port + "/"
	stop, err := server([]string{self}, []string{"CHARTER_TEST_SERVER=parent", "PORT=" + port}, url)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	stop() // a second stop is nothing
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		res, err := http.Get(url)
		if err != nil {
			return
		}
		res.Body.Close()
		if time.Now().After(deadline) {
			t.Fatalf("%s still answers after the server was stopped: what it started is still running", url)
		}
	}
}

func TestMain(m *testing.M) {
	switch os.Getenv("CHARTER_TEST_SERVER") {
	case "parent":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), "CHARTER_TEST_SERVER=child")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		child.Run()
		return
	case "child":
		http.ListenAndServe("localhost:"+os.Getenv("PORT"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		return
	}
	os.Exit(m.Run())
}

// A task line is read by sh on macOS and Linux and by cmd.exe on Windows, so it has only what both
// take the same way: programs and their arguments, double quotes, &&. What only sh has is a
// command of the tool (exec, lint, with-server) or a template of mise's.
func TestTaskLinesNeedNoUnixShell(t *testing.T) {
	files := []string{"../../mise.toml"}
	for _, name := range examples(t) {
		files = append(files, filepath.Join("../../examples", name, "mise.toml"))
	}
	run := regexp.MustCompile(`(?m)^run(_windows)? = (.*)$`)
	setting := regexp.MustCompile(`(^|&& )[A-Z_]+=`)
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		lines := run.FindAllStringSubmatch(string(content), -1)
		if len(lines) == 0 {
			t.Errorf("%s: no tasks", file)
		}
		for _, m := range lines {
			line := m[2]
			if m[1] != "" {
				t.Errorf("%s: a line for Windows alone: %s", file, line)
			}
			// The TOML string's own quotes are not the command's.
			if len(line) < 2 || line[0] != line[len(line)-1] || strings.HasPrefix(line, `"""`) || strings.HasPrefix(line, "'''") {
				t.Errorf("%s: not a one-line string: %s", file, line)
				continue
			}
			command := line[1 : len(line)-1]
			for unix, instead := range map[string]string{
				"$":            "a setting is {{env.NAME}}",
				"'":            "cmd.exe keeps single quotes: use double quotes",
				"node_modules": "charter exec finds an npm program",
				">":            "charter exec -quiet",
				"|":            "a command of the tool",
				";":            "&&",
				"`":            "a command of the tool",
				"%":            "a setting is {{env.NAME}}",
			} {
				if strings.Contains(command, unix) {
					t.Errorf("%s: %q is not the same in sh and cmd.exe (%s): %s", file, unix, instead, command)
				}
			}
			if setting.MatchString(command) {
				t.Errorf("%s: NAME=value before a program is sh only (charter exec -env NAME=value): %s", file, command)
			}
		}
	}
}
