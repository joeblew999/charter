//go:build unix || windows

package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

	until("charter dev serves the page", "/", "<h1>Messages</h1>")
	until("its scripts are Vite's dev server's", "/", `src="/__vite/web/main.js"`)
	until("a variable is wrangler.jsonc's", "/api/hello", "Hello from charter-site")

	edit("api/handlers.go", `"Hello from "`, `"Greetings from "`)
	until("a Go edit", "/api/hello", "Greetings from charter-site")

	edit("pages/home.gsx", `class="flex flex-col gap-2"`, `class="flex flex-col gap-3"`)
	until("a gsx edit", "/", `class="flex flex-col gap-3"`)

	edit("locales/de.toml", `heading = "Nachrichten"`, `heading = "Pinnwand"`)
	until("a translation alone", "/de/", "<h1>Pinnwand</h1>")

	edit("wrangler.jsonc", `"APP_NAME": "charter-site"`, `"APP_NAME": "charter-site, renamed"`)
	until("a variable in wrangler.jsonc", "/api/hello", "Greetings from charter-site, renamed")

	// A build that fails keeps the last version that worked; mending it brings the new one.
	broken, err := os.ReadFile("api/handlers.go")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile("api/handlers.go", append(broken, "\nfunc broken( {\n"...), 0o644)
	time.Sleep(8 * time.Second)
	until("a build that fails keeps the last one serving", "/api/hello", "Greetings from charter-site, renamed")
	os.WriteFile("api/handlers.go", []byte(strings.Replace(string(broken), `"Greetings from "`, `"Mended, from "`, 1)), 0o644)
	until("mending it", "/api/hello", "Mended, from charter-site, renamed")

	// Another command beside it: charter check passes, and dev is not disturbed.
	check := exec.Command("go", "tool", "charter", "check")
	if b, err := check.CombinedOutput(); err != nil {
		t.Fatalf("✗ charter check beside charter dev: %v\n%s", err, b)
	}
	until("charter check beside it leaves dev serving", "/api/hello", "Mended, from charter-site, renamed")
}
