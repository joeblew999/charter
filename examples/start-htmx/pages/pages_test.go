package pages

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/examples/start-htmx/api"
)

// site is the pages over the API, as `go run .` serves them, on one in-memory store and hub.
func site(t *testing.T) (*httptest.Server, api.Env) {
	t.Helper()
	memory := &api.MemStore{}
	env := api.Env{
		Var:   func(string) string { return "test" },
		Store: func() (api.Store, error) { return memory, nil },
		Hub:   func() (api.Hub, error) { return memory, nil },
	}
	srv := httptest.NewServer(Handler(env, api.Handler(env)))
	t.Cleanup(srv.Close)
	return srv, env
}

func get(t *testing.T, url string) (int, string, http.Header) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body), res.Header
}

func postForm(t *testing.T, srv *httptest.Server, body string) (int, string) {
	t.Helper()
	res, err := http.PostForm(srv.URL+"/messages", url.Values{"body": {body}})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(out)
}

func TestTheHomePageShowsTheMessagesNewestFirstAndFollowsFromTheNewest(t *testing.T) {
	srv, env := site(t)
	status, page, header := get(t, srv.URL+"/")
	if status != 200 || header.Get("Content-Type") != "text/html; charset=utf-8" || !strings.HasPrefix(page, "<!DOCTYPE html>") {
		t.Fatalf("HTTP %d %s\n%s", status, header.Get("Content-Type"), page)
	}
	for _, want := range []string{`<script src="/static/htmx-4.0.0.min.js">`, `<script src="/static/hx-sse-4.0.0.min.js">`, `<form id="post" hx-post="/messages"`, `hx-sse:connect="/messages/stream?after=0"`} {
		if !strings.Contains(page, want) {
			t.Errorf("no %s in\n%s", want, page)
		}
	}
	for _, body := range []string{"first", "second"} {
		if _, err := env.Post(context.Background(), body); err != nil {
			t.Fatal(err)
		}
	}
	_, page, _ = get(t, srv.URL+"/")
	second, first := strings.Index(page, `<li id="message-2"><span>second</span>`), strings.Index(page, `<li id="message-1"><span>first</span>`)
	if second < 0 || first < second || !strings.Contains(page, `hx-sse:connect="/messages/stream?after=2"`) {
		t.Errorf("not the two messages, newest first, followed from 2:\n%s", page)
	}
}

// The form posts as the API does, and is answered with itself: empty, or with what was refused and
// why (422, which htmx 4 swaps in too). The message reaches every page, this one too, by the stream.
func TestTheFormPostsAndIsAnsweredWithItself(t *testing.T) {
	srv, env := site(t)
	status, form := postForm(t, srv, "  hello  ")
	if status != 200 || !strings.HasPrefix(form, `<form id="post"`) || !strings.Contains(form, `value=""`) || strings.Contains(form, "<li") {
		t.Fatalf("HTTP %d %s", status, form)
	}
	if messages, _ := env.Recent(context.Background(), 10); len(messages) != 1 || messages[0].Body != "hello" {
		t.Errorf("stored: %v", messages)
	}
	status, form = postForm(t, srv, "   ")
	if status != 422 || !strings.Contains(form, `<p class="problem">a message is 1 to 280 characters`) {
		t.Errorf("an empty message: HTTP %d %s", status, form)
	}
	if messages, _ := env.Recent(context.Background(), 10); len(messages) != 1 {
		t.Errorf("an empty message was stored: %v", messages)
	}
}

// stream opens the stream with these headers and returns a function that reads its next event.
func stream(t *testing.T, address string, header http.Header) func() string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", address, nil)
	req.Header = header
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("HTTP %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	t.Cleanup(func() { res.Body.Close() })
	events := make(chan string, 10)
	go func() {
		lines := bufio.NewScanner(res.Body)
		var event strings.Builder
		for lines.Scan() {
			if lines.Text() != "" {
				event.WriteString(lines.Text() + "\n")
				continue
			}
			if !strings.HasPrefix(event.String(), ":") {
				events <- event.String()
			}
			event.Reset()
		}
	}()
	return func() string {
		select {
		case event := <-events:
			return event
		case <-time.After(5 * time.Second):
			t.Fatal("no event in 5 s")
			return ""
		}
	}
}

// Every message posted, from a page or the API, reaches every open stream as the Item fragment,
// escaped; a stream that connects again with Last-Event-ID goes on from there.
func TestTheStreamSendsEachNewMessageAsItsFragment(t *testing.T) {
	srv, env := site(t)
	if _, err := env.Post(context.Background(), "before the page"); err != nil {
		t.Fatal(err)
	}
	next := stream(t, srv.URL+"/messages/stream?after=1", nil)
	postForm(t, srv, "from the form <b>bold</b>")
	// An event with no name (htmx swaps its data in), one data line: the fragment.
	if event := next(); !strings.HasPrefix(event, "id: 2\ndata: <li id=\"message-2\"><span>from the form &lt;b&gt;bold&lt;/b&gt;</span><time>") || strings.Count(event, "\n") != 2 || !strings.HasSuffix(event, "</time></li>\n") {
		t.Errorf("event:\n%s", event)
	}
	if status, body := postJSON(t, srv, `{"body":"from the API"}`); status != 200 {
		t.Fatalf("POST /api/messages: HTTP %d %s", status, body)
	}
	if event := next(); !strings.HasPrefix(event, "id: 3\ndata: <li id=\"message-3\"><span>from the API</span>") {
		t.Errorf("event:\n%s", event)
	}
	// A browser that comes back sends the last id it had: the URL's after is older, Last-Event-ID wins.
	again := stream(t, srv.URL+"/messages/stream?after=1", http.Header{"Last-Event-ID": {"2"}})
	if event := again(); !strings.HasPrefix(event, "id: 3\n") {
		t.Errorf("resumed after 2:\n%s", event)
	}
}

func postJSON(t *testing.T, srv *httptest.Server, body string) (int, string) {
	t.Helper()
	res, err := http.Post(srv.URL+"/api/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(out)
}

// A message with line breaks is still one event: every line of its fragment is a data line.
func TestALineBreakIsADataLine(t *testing.T) {
	event, err := htmxEvent(context.Background(), api.Message{ID: 7, Body: "one\r\ntwo\rthree\nfour"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "data: <li id=\"message-7\"><span>one\ndata: two\ndata: three\ndata: four</span><time></time></li>"; event != want {
		t.Errorf("got\n%s\nwant\n%s", event, want)
	}
}

func TestStaticFiles(t *testing.T) {
	srv, _ := site(t)
	for path, want := range map[string]string{
		"/static/htmx-4.0.0.min.js":   "var htmx=",
		"/static/hx-sse-4.0.0.min.js": "registerExtension(\"sse\"",
		"/static/site.css":            "main {",
	} {
		status, body, header := get(t, srv.URL+path)
		if status != 200 || !strings.Contains(body, want) || header.Get("Cache-Control") == "" {
			t.Errorf("%s: HTTP %d, Cache-Control %q", path, status, header.Get("Cache-Control"))
		}
	}
	if status, _, _ := get(t, srv.URL+"/static/nope.js"); status != 404 {
		t.Errorf("/static/nope.js: HTTP %d", status)
	}
	// Every other path is the API's.
	if status, body, _ := get(t, srv.URL+"/api/hello"); status != 200 || !strings.Contains(body, "Hello from test") {
		t.Errorf("/api/hello: HTTP %d %s", status, body)
	}
}
