package test

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/env"
)

// A reader sees their own writes, and never the database earlier than they have seen it, though reads may be
// answered by a copy of the database that is behind the one that takes the writes: the answer to a request that
// read or wrote gives a bookmark, and the next request brings it back. The app has
// no line about any of it (cloud/session.go). These ask a real Worker, with plain HTTP, since it is the headers
// that are tested.

// asked is one request to the Worker, and its answer with the body read.
func asked(t *testing.T, method, path, body string, headers ...string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, apptest.Base(t)+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	// A redirect is an answer to look at, not to follow.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	text, _ := io.ReadAll(res.Body)
	return res, string(text)
}

func bookmarkCookie(res *http.Response) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == cloud.BookmarkCookie {
			return c
		}
	}
	return nil
}

var aBookmark = regexp.MustCompile(`^DB=[0-9a-f]{8}(-[0-9a-f]{8}){2}-[0-9a-f]{32}$`)

// A write is answered with the bookmark of what it wrote, as a header and as a cookie; the request that brings it
// back reads the note, and is never given an older one.
func TestAWriteGivesABookmarkAndTheNextRequestSeesIt(t *testing.T) {
	writer := "Bearer " + apptest.Secret(t, env.WRITE_TOKEN)
	res, body := asked(t, http.MethodPost, "/api/notes", `{"body":"read your own write"}`, "Authorization", writer)
	if res.StatusCode/100 != 2 {
		t.Fatalf("writing a note: %s: %s", res.Status, body)
	}
	var note struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &note); err != nil || note.ID == 0 {
		t.Fatalf("the note written: %s", body)
	}
	t.Cleanup(func() {
		asked(t, http.MethodDelete, "/api/notes/"+strconv.FormatInt(note.ID, 10), "", "Authorization", "Bearer "+apptest.Secret(t, env.ADMIN_TOKEN))
	})
	given := res.Header.Get(cloud.BookmarkHeader)
	if !aBookmark.MatchString(given) {
		t.Fatalf("the write's %s is %q, want DB=<a bookmark>", cloud.BookmarkHeader, given)
	}
	c := bookmarkCookie(res)
	if c == nil || c.Value != given || !c.HttpOnly || c.MaxAge != cloud.BookmarkLasts {
		t.Fatalf("the write's cookie: %+v, want %s with the same bookmark", c, cloud.BookmarkCookie)
	}

	// A program brings the header back, a browser the cookie: each reads the note, and is given nothing unless the
	// database has moved on since, and then where it is now.
	for name, carry := range map[string][]string{"header": {cloud.BookmarkHeader, given}, "cookie": {"Cookie", c.Name + "=" + c.Value}} {
		res, body := asked(t, http.MethodGet, "/api/notes", "", carry...)
		if res.StatusCode != http.StatusOK || !strings.Contains(body, "read your own write") {
			t.Errorf("with the %s: %s: %s", name, res.Status, body)
		}
		if again := res.Header.Get(cloud.BookmarkHeader); again != "" && again <= given {
			t.Errorf("with the %s: given %q, which is no later than the %q brought", name, again, given)
		}
	}
}

// A request that only read is given where the database was when it read, on a page and on the API: the reader's
// next request, answered by a copy that is behind, waits for it, so nothing they were shown is gone again. Brought
// back, it is answered with nothing, or with where the database has moved on to.
func TestAReadGivesItsBookmark(t *testing.T) {
	for _, path := range []string{"/", "/api/notes"} {
		res, _ := asked(t, http.MethodGet, path, "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: %s", path, res.Status)
		}
		given := res.Header.Get(cloud.BookmarkHeader)
		c := bookmarkCookie(res)
		if !aBookmark.MatchString(given) || c == nil || c.Value != given {
			t.Fatalf("GET %s: %s is %q and the cookie %+v, want DB=<a bookmark> in both", path, cloud.BookmarkHeader, given, c)
		}
		again, _ := asked(t, http.MethodGet, path, "", "Cookie", c.Name+"="+c.Value)
		if later := again.Header.Get(cloud.BookmarkHeader); later != "" && later <= given {
			t.Errorf("GET %s with its bookmark: given %q, which is no later than the %q brought", path, later, given)
		}
	}
}

// A page that reads no database is given nothing: no cookie, no header.
func TestAPageWithoutADatabaseSetsNoCookie(t *testing.T) {
	res, _ := asked(t, http.MethodGet, "/docs/", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /docs/: %s", res.Status)
	}
	if bookmarkCookie(res) != nil || res.Header.Get(cloud.BookmarkHeader) != "" {
		t.Errorf("GET /docs/ was given a bookmark: %q", res.Header.Get(cloud.BookmarkHeader))
	}
}

// What is brought as a bookmark and is none costs the request nothing: one that is malformed starts nothing, and
// one of the right shape that D1 does not know (another database's, one too old to be kept) is given up for the
// primary.
func TestABadBookmarkStillAnswers(t *testing.T) {
	for name, value := range map[string]string{
		"malformed":        "DB=first-primary; DROP TABLE notes",
		"not this binding": "NOPE=0000007b-0000b26e-00001538-0c3e87bb37b3db5cc52eedb93cd3b96b",
		"unknown to D1":    "DB=ffffffff-ffffffff-ffffffff-ffffffffffffffffffffffffffffffff",
		"from long ago":    "DB=00000000-00000000-00000000-00000000000000000000000000000001",
	} {
		for _, path := range []string{"/", "/api/notes"} {
			res, body := asked(t, http.MethodGet, path, "", cloud.BookmarkHeader, value)
			if res.StatusCode != http.StatusOK {
				t.Errorf("%s bookmark, GET %s: %s: %.200s", name, path, res.Status, body)
			}
		}
	}
}

// Who answered a request's statements is said to who asks which runtime answered: a region and whether it is the
// primary on Cloudflare, "local" under wrangler dev, where there is one database and no region.
func TestWhoAnsweredIsSaidWhenAsked(t *testing.T) {
	res, _ := asked(t, http.MethodGet, "/api/notes", "", "x-go-runtime", "?")
	served := res.Header.Get("X-D1-Served")
	want := regexp.MustCompile(`^DB [A-Z]+ (primary|replica)(, DB [A-Z]+ (primary|replica))*$`)
	if apptest.Local() {
		want = regexp.MustCompile(`^DB local(, DB local)*$`)
	}
	if !want.MatchString(served) {
		t.Errorf("X-D1-Served: %q", served)
	}
	if res, _ := asked(t, http.MethodGet, "/api/notes", ""); res.Header.Get("X-D1-Served") != "" {
		t.Error("who answered was said unasked")
	}
}

// Who someone is is read at the primary, and the rest of their request from there on: a caller with a token is
// never answered by a copy that is behind. (Under wrangler dev there is only "local" to see.)
func TestACallerIsIdentifiedAtThePrimary(t *testing.T) {
	if apptest.Local() {
		t.Skip("one database and no primary under wrangler dev")
	}
	res, _ := asked(t, http.MethodGet, "/api/people", "", "Authorization", "Bearer "+apptest.Secret(t, env.ADMIN_TOKEN), "x-go-runtime", "?")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/people: %s", res.Status)
	}
	first, _, _ := strings.Cut(res.Header.Get("X-D1-Served"), ",")
	if !strings.HasSuffix(first, " primary") {
		t.Errorf("the first statement of a request with a token was answered by %q (all: %q)", first, res.Header.Get("X-D1-Served"))
	}
}

// A call to the API that brings no bookmark is answered by the primary: a program that writes and then reads sees
// what it wrote without knowing of bookmarks, which a generated client does not. One that brings back the bookmark
// it was given may be answered by a copy.
func TestAnAPICallWithoutABookmarkReadsThePrimary(t *testing.T) {
	if apptest.Local() {
		t.Skip("one database and no primary under wrangler dev")
	}
	res, _ := asked(t, http.MethodGet, "/api/notes", "", "x-go-runtime", "?")
	if served := res.Header.Get("X-D1-Served"); served == "" || strings.Contains(served, "replica") {
		t.Errorf("without a bookmark: X-D1-Served %q, want the primary only", served)
	}
}

// A part of a page that asks for itself because its feed said something changed reads the primary: the request
// says so (live.Follows), and a copy that is behind would show the page as it was before the change.
func TestARequestThatSaysNewestReadsThePrimary(t *testing.T) {
	if apptest.Local() {
		t.Skip("one database and no primary under wrangler dev")
	}
	res, _ := asked(t, http.MethodGet, "/", "", cloud.NewestHeader, "1", "x-go-runtime", "?")
	if served := res.Header.Get("X-D1-Served"); served == "" || strings.Contains(served, "replica") {
		t.Errorf("with %s: X-D1-Served %q, want the primary only", cloud.NewestHeader, served)
	}
}

// The Go that starts as the Worker's module loads is there for the first request. When it is not (Cloudflare
// refused something it did there, or its own runtime ended it), the Worker says so to whoever asks which runtime
// answered, and starts another for each first request: every request is still answered, later, and nothing else
// shows it. A local Worker allows while its module loads what Cloudflare refuses, so it is the deployed one this
// is for.
func TestTheWorkersGoStartedWithIt(t *testing.T) {
	res, _ := asked(t, http.MethodGet, "/api/hello", "", "x-go-runtime", "?")
	said := res.Header.Get("x-go-runtime")
	if said == "" {
		t.Fatal("the Worker does not say which runtime answered (x-go-runtime)")
	}
	if strings.Contains(said, "failed") {
		t.Errorf("the Go that started with the Worker did not last: %s", said)
	}
}
