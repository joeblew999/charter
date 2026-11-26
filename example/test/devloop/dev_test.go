//go:build unix || windows

package devloop

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestDev holds charter dev to what it is for: it stays running, and everything a developer does meanwhile reaches
// the open page by itself. It starts charter dev here, then, as a second terminal would, edits Go, a component, a
// translation and a variable of the Worker, breaks the build and mends it, and runs charter check beside it; after
// each it waits for the page to say so. It needs Node and the packages installed (npm install at the repo's top), so
// it runs only when CHARTER_DEV_TEST is set, as the workflow's dev job does on macOS, Linux and Windows.
func TestDev(t *testing.T) {
	if os.Getenv("CHARTER_DEV_TEST") == "" {
		t.Skip("CHARTER_DEV_TEST is not set: this starts charter dev and edits the example's files")
	}
	t.Chdir("../..") // the app's folder
	// What charter dev writes beside the catalogs as they change is put back last, after it has stopped.
	if sum, err := os.ReadFile("locales/intl.sum"); err == nil {
		t.Cleanup(func() { os.WriteFile("locales/intl.sum", sum, 0o644) })
	}
	dev := exec.Command("go", "tool", "charter", "dev")
	own(dev)
	out, err := dev.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	dev.Stderr = dev.Stdout
	if err := dev.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stop(dev) })

	// The address gsx dev prints: "open http://localhost:5173".
	found := make(chan string, 1)
	var log strings.Builder
	go func() {
		lines := bufio.NewScanner(out)
		lines.Buffer(make([]byte, 1<<20), 1<<20)
		open := regexp.MustCompile(`open (http://localhost:\d+)`)
		for lines.Scan() {
			log.WriteString(lines.Text() + "\n")
			if m := open.FindStringSubmatch(lines.Text()); m != nil {
				select {
				case found <- m[1]:
				default:
				}
			}
		}
	}()
	var base string
	select {
	case base = <-found:
	case <-time.After(5 * time.Minute):
		t.Fatalf("charter dev never said where it serves:\n%s", log.String())
	}
	page := func(path string) string {
		resp, err := http.Get(base + path)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return string(b)
	}
	// until waits for path to have want, as a developer waits for the browser to show it.
	until := func(what, path, want string) {
		t.Helper()
		start := time.Now()
		for time.Since(start) < 2*time.Minute {
			if strings.Contains(page(path), want) {
				t.Logf("✓ %s: on the page after %.1f s", what, time.Since(start).Seconds())
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatalf("✗ %s: %s never had %q\n%s\n--- charter dev said:\n%s", what, path, want, page(path), log.String())
	}
	// edit replaces old with new in file, and puts the file back when the test ends.
	edit := func(file, old, new string) {
		t.Helper()
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), old) {
			t.Fatalf("%s has no %q to edit", file, old)
		}
		t.Cleanup(func() { os.WriteFile(file, b, 0o644) })
		if err := os.WriteFile(file, []byte(strings.Replace(string(b), old, new, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	until("charter dev serves the page", "/", "<h1>Notes</h1>")
	until("its scripts are Vite's dev server's", "/", `src="/__vite/web/main.js"`)
	until("a variable is wrangler.jsonc's", "/api/hello", "Hello from charter-example")

	edit("api/handlers.go", `"Hello from "`, `"Greetings from "`)
	until("a Go edit", "/api/hello", "Greetings from charter-example")

	edit("pages/home.gsx", `class="flex flex-col gap-2"`, `class="flex flex-col gap-3"`)
	until("a gsx edit", "/", `class="flex flex-col gap-3"`)

	edit("locales/de.toml", `heading = "Notizen"`, `heading = "Pinnwand"`)
	until("a translation alone", "/de/", "<h1>Pinnwand</h1>")

	edit("wrangler.jsonc", `"APP_NAME": "charter-example"`, `"APP_NAME": "charter-example, renamed"`)
	until("a variable in wrangler.jsonc", "/api/hello", "Greetings from charter-example, renamed")

	// The docs: a page of docs/ is Markdown, and a save shows.
	edit("docs/index.md", "# The example\n", "# The example, edited\n")
	until("an edit to the docs", "/docs/", "The example, edited</h1>")

	// The schema: a table added to db/schema.sql is in the local database a moment later (its migration written,
	// and applied), with nothing run by hand.
	t.Cleanup(func() {
		pending, _ := filepath.Glob("db/migrations/*create_scratch*.sql")
		for _, f := range pending {
			os.Remove(f)
		}
	})
	edit("db/schema.sql", "CREATE TABLE notes (", "CREATE TABLE scratch (id INTEGER PRIMARY KEY, label TEXT);\n\nCREATE TABLE notes (")
	for start := time.Now(); ; time.Sleep(time.Second) {
		out, _ := exec.Command("go", "tool", "charter", "run", "db:execute", "SELECT name FROM sqlite_master WHERE name = 'scratch'").CombinedOutput()
		if strings.Contains(string(out), `"name": "scratch"`) {
			t.Log("✓ a table added to db/schema.sql is in the local database")
			break
		}
		// The first time on a machine, the programs that read the schema are built first: minutes, once.
		if time.Since(start) > 4*time.Minute {
			t.Fatalf("✗ the schema's new table never reached the local database\n%s\n%s", out, log.String())
		}
	}

	// A build that fails keeps the last version that worked; mending it brings the new one.
	broken, err := os.ReadFile("api/handlers.go")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile("api/handlers.go", append(broken, "\nfunc broken( {\n"...), 0o644)
	time.Sleep(8 * time.Second)
	until("a build that fails keeps the last one serving", "/api/hello", "Greetings from charter-example, renamed")
	os.WriteFile("api/handlers.go", []byte(strings.Replace(string(broken), `"Greetings from "`, `"Mended, from "`, 1)), 0o644)
	until("mending it", "/api/hello", "Mended, from charter-example, renamed")

	// A catalog that does not compile is a build that fails, said where a developer is looking: gsx's overlay gets
	// what is wrong (charter dev keeps it for the build to fail with), the page stays, and mending it brings the new text.
	catalog, err := os.ReadFile("locales/de.toml")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile("locales/de.toml", []byte(strings.Replace(string(catalog), `heading = "Pinnwand"`, `heading = "{count, plural, one {eine"`, 1)), 0o644)
	for start := time.Now(); ; time.Sleep(200 * time.Millisecond) {
		if b, err := os.ReadFile(".charter/dev/messages.err"); err == nil && len(b) > 0 && strings.Contains(log.String(), "de.toml home.heading") {
			t.Logf("✓ a catalog that does not compile fails the build, with what is wrong, after %.1f s", time.Since(start).Seconds())
			break
		}
		if time.Since(start) > 60*time.Second {
			t.Fatalf("✗ a broken catalog never failed the build\n%s", log.String())
		}
	}
	until("a broken catalog keeps the last page serving", "/de/", "<h1>Pinnwand</h1>")
	os.WriteFile("locales/de.toml", []byte(strings.Replace(string(catalog), `heading = "Pinnwand"`, `heading = "Zettel"`, 1)), 0o644)
	until("mending the catalog", "/de/", "<h1>Zettel</h1>")

	// What is running is written down for the other commands, and for an agent: charter env says where.
	env, _ := exec.Command("go", "tool", "charter", "env").CombinedOutput()
	if !strings.Contains(string(env), "dev        running at "+base) {
		t.Errorf("✗ charter env does not say where dev is running:\n%s", env)
	} else {
		t.Log("✓ charter env says where dev is running, and where its log is")
	}
	if b, err := os.ReadFile(".charter/dev/dev.log"); err != nil || len(b) == 0 {
		t.Errorf("✗ no log of the Worker and the builds at .charter/dev/dev.log: %v", err)
	}

	// Another command beside it: charter check passes, and dev is not disturbed.
	check := exec.Command("go", "tool", "charter", "check")
	if b, err := check.CombinedOutput(); err != nil {
		t.Fatalf("✗ charter check beside charter dev: %v\n%s", err, b)
	}
	until("charter check beside it leaves dev serving", "/api/hello", "Mended, from charter-example, renamed")
}
