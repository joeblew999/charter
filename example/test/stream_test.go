package test

import (
	"bufio"
	"context"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/sdk/go/option"
)

// A page's stream says one thing, that what the page shows has changed, and nothing is lost when it breaks: a page
// says when it was rendered, a stream that connects again says when it was last told, and the hub answers
// "changed" at once when something changed since, or when it cannot know. Every stream breaks when the Worker is
// deployed, which restarts the hub.

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

// A stream that opens for a page rendered a while after the last change says nothing yet but when the page was:
// an id alone, which the page says if it must connect again before anything changes.
func TestAStreamOpensWithWhenThePageWas(t *testing.T) {
	// A change now, so that the hub knows of one; then a page "rendered" a minute after it.
	writer := sdk(t, option.WithToken(apptest.Secret(t, "WRITE_TOKEN")))
	write(t, writer, "a change the hub knows of")
	later := strconv.FormatInt(time.Now().Add(time.Minute).UnixMilli(), 10)
	lines := streamed(t, 1, "Last-Event-ID", later)
	if len(lines) != 1 || lines[0] != "id: "+later {
		t.Fatalf("the stream's first line: %q, want the id %s alone", lines, later)
	}
}

// A stream for a page that was rendered before the last change is told at once that something changed: what
// happened between the page's rendering and its stream's opening, or while its stream was broken, is not lost.
func TestAStreamBehindIsToldAtOnce(t *testing.T) {
	// Long ago: the first millisecond there was.
	lines := streamed(t, 2, "Last-Event-ID", "1")
	if len(lines) != 2 || !regexp.MustCompile(`^id: \d+$`).MatchString(lines[0]) || lines[1] != "event: changed" {
		t.Errorf("a stream that is behind: %q, want an id and the event changed", lines)
	}
}
