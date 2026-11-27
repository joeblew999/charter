package test

import (
	"bufio"
	"context"
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
)

// A page's stream says nothing is lost: a page that connects again says where it was, and is told at once to ask
// for what came after. For that to hold before any item has come (the hub restarts on a deploy, and every open
// page's stream breaks), the stream gives the page's position as its first id.

// The page's own stream: the address its markup connects to.
var pageStream = regexp.MustCompile(`hx-sse:connect="([^"]+)"`)

// streamed opens the home page's stream and gives what it sends until wanted lines have come, or two seconds pass.
func streamed(t *testing.T, wanted int, headers ...string) []string {
	t.Helper()
	found := pageStream.FindStringSubmatch(apptest.Get(t, "/"))
	if found == nil {
		t.Fatal("the home page has no stream (hx-sse:connect)")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apptest.Base(t)+html.UnescapeString(found[1]), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("the stream: %s, %s", res.Status, res.Header.Get("Content-Type"))
	}
	var lines []string
	for scanner := bufio.NewScanner(res.Body); len(lines) < wanted && scanner.Scan(); {
		if line := scanner.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// A stream that has just opened gives the page's position as an id, alone: what the page says if it must connect
// again before anything is published.
func TestAStreamOpensWithThePagesPosition(t *testing.T) {
	lines := streamed(t, 1)
	if len(lines) != 1 || !regexp.MustCompile(`^id: \d+$`).MatchString(lines[0]) {
		t.Fatalf("the stream's first line: %q, want an id alone", lines)
	}
}

// A stream that connects again, saying where the page was, is told at once to ask for what came after: an element
// that fetches it.
func TestAStreamThatConnectsAgainCatchesUp(t *testing.T) {
	lines := streamed(t, 3, "Last-Event-ID", "0")
	if len(lines) != 3 {
		t.Fatalf("connected again: %q, want the position and then what to fetch", lines)
	}
	if lines[0] != "id: 0" || lines[1] != "id: 0" || !strings.Contains(lines[2], `hx-get="`) || !strings.Contains(lines[2], `after=0"`) {
		t.Errorf("connected again: %q", lines)
	}
}
