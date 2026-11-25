// Package test asks the Worker, on wrangler dev, for each thing workers-go gives it: BASE is its URL (go tool
// charter test sets it).
package test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var base = strings.TrimRight(os.Getenv("BASE"), "/")

func TestMain(m *testing.M) {
	if base == "" {
		fmt.Println("no BASE, the Worker's URL: go tool charter test")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func do(t *testing.T, method, path, body string) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "text/plain")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b), res.Header
}

// want asks once and holds the answer to a status and a body.
func want(t *testing.T, method, path, body string, status int, answer string) {
	t.Helper()
	if s, b, _ := do(t, method, path, body); s != status || (answer != "" && b != answer) {
		t.Errorf("%s %s: HTTP %d %q, want %d %q", method, path, s, b, status, answer)
	}
}

// soon waits for what happens after an answer: a queue's consumer, waitUntil, a Cron Trigger.
func soon(t *testing.T, path, answer string) {
	t.Helper()
	var s int
	var b string
	for start := time.Now(); time.Since(start) < 30*time.Second; time.Sleep(300 * time.Millisecond) {
		if s, b, _ = do(t, "GET", path, ""); s == 200 && b == answer {
			return
		}
	}
	t.Errorf("GET %s: HTTP %d %q after 30 s, want %q", path, s, b, answer)
}

func TestVariables(t *testing.T) { want(t, "GET", "/env", "", 200, "charter-workers") }

func TestKV(t *testing.T) {
	want(t, "PUT", "/kv/greeting", "hello", 200, "")
	want(t, "GET", "/kv/greeting", "", 200, "hello")
}

func TestR2(t *testing.T) {
	want(t, "PUT", "/r2/note.txt", "kept in a bucket", 200, "")
	s, b, h := do(t, "GET", "/r2/note.txt", "")
	if s != 200 || b != "kept in a bucket" || !strings.HasPrefix(h.Get("Content-Type"), "text/plain") {
		t.Errorf("GET /r2/note.txt: HTTP %d %q %s", s, b, h.Get("Content-Type"))
	}
	want(t, "GET", "/r2/nothing", "", 404, "")
}

func TestQueueProducerAndConsumer(t *testing.T) {
	want(t, "POST", "/queue/first", "", 200, "")
	soon(t, "/kv/queue:first", "consumed")
}

func TestD1(t *testing.T) {
	s, b, _ := do(t, "POST", "/d1/ada", "")
	if s != 200 || b == "" || b == "0" {
		t.Errorf("POST /d1/ada: HTTP %d %q, want a count of visitors", s, b)
	}
}

func TestCache(t *testing.T) {
	if s, b, h := do(t, "GET", "/cache", ""); s != 200 || b != "cached" || h.Get("X-Cache") != "miss" {
		t.Errorf("first: HTTP %d %q X-Cache %q, want a miss", s, b, h.Get("X-Cache"))
	}
	if s, b, h := do(t, "GET", "/cache", ""); s != 200 || b != "cached" || h.Get("X-Cache") != "hit" {
		t.Errorf("second: HTTP %d %q X-Cache %q, want a hit", s, b, h.Get("X-Cache"))
	}
}

func TestDurableObjectStub(t *testing.T) {
	if s, b, _ := do(t, "GET", "/do", ""); s != 200 || len(b) != 3 {
		t.Errorf("GET /do: HTTP %d %q, want the hub's status", s, b)
	}
}

func TestFetch(t *testing.T) { want(t, "GET", "/fetch?url="+base+"/healthz", "", 200, "ok") }

func TestSocket(t *testing.T) {
	addr := strings.TrimPrefix(base, "http://")
	if s, b, _ := do(t, "GET", "/socket?addr="+addr, ""); s != 200 || !strings.HasPrefix(b, "HTTP/1.") {
		t.Errorf("GET /socket: HTTP %d %q, want the first line of an HTTP answer", s, b)
	}
}

func TestWaitUntil(t *testing.T) {
	want(t, "POST", "/wait-until", "", 200, "answered")
	soon(t, "/kv/wait-until", "done")
}

func TestCronTrigger(t *testing.T) {
	// wrangler dev --test-scheduled fires one by hand.
	if s, _, _ := do(t, "GET", "/__scheduled?cron=*/5+*+*+*+*", ""); s != 200 {
		t.Fatalf("GET /__scheduled: HTTP %d", s)
	}
	soon(t, "/kv/cron", "*/5 * * * *")
}

func TestRequestProperties(t *testing.T) {
	if s, b, _ := do(t, "GET", "/cf", ""); s != 200 || strings.TrimSpace(b) == "" {
		t.Errorf("GET /cf: HTTP %d %q, want what Cloudflare knows of the request", s, b)
	}
}

func TestEmailSend(t *testing.T) { want(t, "POST", "/email/you@example.com", "", 200, "") }

func TestEmailReceive(t *testing.T) {
	// wrangler dev hands the Worker a message posted here, as Email Routing would.
	raw := "From: ada@example.com\r\nTo: app@example.com\r\nSubject: A note for the Worker\r\nMessage-ID: <note@example.com>\r\n\r\nHello.\r\n"
	if s, b, _ := do(t, "POST", "/cdn-cgi/handler/email?from=ada@example.com&to=app@example.com", raw); s/100 != 2 {
		t.Fatalf("POST /cdn-cgi/handler/email: HTTP %d %s", s, b)
	}
	soon(t, "/kv/email:ada@example.com", "A note for the Worker")
}
