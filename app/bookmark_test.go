package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joeblew999/charter/cloud"
)

const (
	olderBookmark = "0000007b-0000b26e-00001538-0c3e87bb37b3db5cc52eedb93cd3b96b"
	newerBookmark = "00000085-0000024c-00004c6d-8e61117bf38d7adb71b934ebbf891683"
)

// answered serves one request through carrying, with what the request's statements wrote and who answered them.
func answered(r *http.Request, wrote cloud.Bookmarks, served []string, handle http.HandlerFunc) *http.Response {
	rec := httptest.NewRecorder()
	w := carrying(rec, r, func() cloud.Bookmarks { return wrote }, func() []string { return served })
	handle(w, r)
	w.give()
	return rec.Result()
}

func cookieOf(res *http.Response) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == cloud.BookmarkCookie {
			return c
		}
	}
	return nil
}

// A request that only read is given nothing: no cookie, no header, so its answer can be cached as before.
func TestAReadGivesNoBookmark(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "https://app.example/", nil)
	r.AddCookie(&http.Cookie{Name: cloud.BookmarkCookie, Value: "DB=" + olderBookmark})
	res := answered(r, nil, []string{"DB WEUR replica"}, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("page")) })
	if cookieOf(res) != nil || res.Header.Get(cloud.BookmarkHeader) != "" {
		t.Errorf("a read was given a bookmark: %v", res.Header)
	}
	if res.Header.Get(ServedHeader) != "" {
		t.Errorf("who answered was said unasked: %q", res.Header.Get(ServedHeader))
	}
}

// A request that wrote is given where the database then was, as a cookie for a browser and a header for a program:
// on the redirect that follows a form too, which is where a browser's next request comes from.
func TestAWriteGivesItsBookmark(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "https://app.example/notes", nil)
	res := answered(r, cloud.Bookmarks{"DB": newerBookmark}, nil, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("status %d", res.StatusCode)
	}
	if got := res.Header.Get(cloud.BookmarkHeader); got != "DB="+newerBookmark {
		t.Errorf("header %q", got)
	}
	c := cookieOf(res)
	if c == nil {
		t.Fatal("no cookie on the redirect")
	}
	if c.Value != "DB="+newerBookmark || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.MaxAge != cloud.BookmarkLasts {
		t.Errorf("cookie %+v", c)
	}
	// What the browser sends back is what the next request starts from.
	next := httptest.NewRequest(http.MethodGet, "https://app.example/", nil)
	next.AddCookie(c)
	if got := brought(next); got["DB"] != newerBookmark {
		t.Errorf("the next request brought %v", got)
	}
}

// Of two answers that cross, neither sets the reader back: what is given is never older than what was brought.
func TestTheNewerBookmarkIsKept(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "https://app.example/notes", nil)
	r.AddCookie(&http.Cookie{Name: cloud.BookmarkCookie, Value: "DB=" + newerBookmark})
	res := answered(r, cloud.Bookmarks{"DB": olderBookmark}, nil, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	if c := cookieOf(res); c == nil || c.Value != "DB="+newerBookmark {
		t.Errorf("cookie %+v, want the newer bookmark kept", c)
	}
}

// A handler that returns without writing is given its bookmark too, and http has no cookie for plain http.
func TestNothingWrittenAndPlainHTTP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://localhost:8787/notes", nil)
	res := answered(r, cloud.Bookmarks{"DB": newerBookmark}, nil, func(http.ResponseWriter, *http.Request) {})
	c := cookieOf(res)
	if c == nil || c.Secure {
		t.Errorf("cookie %+v: wanted one, not Secure, on localhost", c)
	}
}

// What a request brought that is no bookmark starts nothing, in the cookie or the header; the header's and the
// cookie's are joined, the later of each.
func TestBrought(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "https://app.example/", nil)
	r.Header.Set(cloud.BookmarkHeader, "DB="+olderBookmark+"&LOG=first-primary")
	r.AddCookie(&http.Cookie{Name: cloud.BookmarkCookie, Value: "DB=" + newerBookmark})
	got := brought(r)
	if len(got) != 1 || got["DB"] != newerBookmark {
		t.Errorf("brought %v", got)
	}
	bad := httptest.NewRequest(http.MethodGet, "https://app.example/", nil)
	bad.Header.Set("Cookie", cloud.BookmarkCookie+"=DB=;drop table")
	if got := brought(bad); len(got) != 0 {
		t.Errorf("a malformed cookie brought %v", got)
	}
}

// Who answered is said to a request that asks which runtime answered, and to no other.
func TestServedIsSaidWhenAsked(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "https://app.example/", nil)
	r.Header.Set("x-go-runtime", "?")
	res := answered(r, nil, []string{"DB APAC primary", "DB WEUR replica"}, func(w http.ResponseWriter, _ *http.Request) { w.Write(nil) })
	if got := res.Header.Get(ServedHeader); got != "DB APAC primary, DB WEUR replica" {
		t.Errorf("%s: %q", ServedHeader, got)
	}
	if strings.Contains(res.Header.Get("Set-Cookie"), cloud.BookmarkCookie) {
		t.Error("a read set the cookie")
	}
}

// A boosted link whose answer is no page is sent there as a navigation, and none of it reaches the page's body; a
// page, and the same answer asked for any other way, goes as it is.
func TestABoostedLinkToNoPageIsANavigation(t *testing.T) {
	file := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("%PDF"))
	}
	boosted := httptest.NewRequest(http.MethodGet, "https://app.example/files/7?x=1", nil)
	boosted.Header.Set("HX-Boosted", "true")
	res := answered(boosted, nil, nil, file)
	if body, _ := io.ReadAll(res.Body); res.Header.Get("HX-Redirect") != "/files/7?x=1" || len(body) != 0 {
		t.Errorf("a boosted link to a file: HX-Redirect %q, body %q", res.Header.Get("HX-Redirect"), body)
	}
	plain := answered(httptest.NewRequest(http.MethodGet, "https://app.example/files/7", nil), nil, nil, file)
	if body, _ := io.ReadAll(plain.Body); plain.Header.Get("HX-Redirect") != "" || string(body) != "%PDF" {
		t.Errorf("the same file asked for plainly: HX-Redirect %q, body %q", plain.Header.Get("HX-Redirect"), body)
	}
	page := answered(boosted, nil, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<body>page</body>"))
	})
	if page.Header.Get("HX-Redirect") != "" {
		t.Errorf("a boosted link to a page: HX-Redirect %q", page.Header.Get("HX-Redirect"))
	}
}
