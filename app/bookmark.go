package app

import (
	"net/http"
	"strings"

	"github.com/joeblew999/charter/cloud"
)

// A reader sees their own writes, with nothing for an app to write: what cloud's sessions need of a request and of
// its answer (cloud/session.go says why and when).
//
// The request brings the bookmark its reader was given: a header for a program, a cookie for a browser. The answer
// gives one only when the request changed a database, so a page that is only read sets no cookie and can be cached
// as before. It is given as the headers go, which is before a handler's body: a statement that runs after that (a
// stream's, or work left for after the answer) is not in it.

// ServedHeader says, to a request that asks which runtime answered (the header x-go-runtime), who answered each of
// its statements on D1: "DB APAC primary, DB WEUR replica".
const ServedHeader = "X-D1-Served"

// brought is the bookmarks a request carries: the later of its header's and its cookie's, for each database.
func brought(r *http.Request) cloud.Bookmarks {
	found := cloud.ParseBookmarks(r.Header.Get(cloud.BookmarkHeader))
	if c, err := r.Cookie(cloud.BookmarkCookie); err == nil {
		found = found.Join(cloud.ParseBookmarks(c.Value))
	}
	return found
}

// carrying is w giving the request's reader, as the headers go, the bookmark of what the request wrote.
func carrying(w http.ResponseWriter, r *http.Request, written func() cloud.Bookmarks, served func() []string) *carried {
	return &carried{ResponseWriter: w, request: r, written: written, served: served}
}

type carried struct {
	http.ResponseWriter
	request *http.Request
	written func() cloud.Bookmarks
	served  func() []string
	given   bool
	sent    bool // answered with where to go, as a navigation: what the handler writes is left out
}

// give sets the answer's headers, once: when the first of them goes, or when the handler has returned having
// written nothing.
func (c *carried) give() {
	if c.given {
		return
	}
	c.given = true
	header := c.Header()
	// A boosted link (Body) whose answer is no page (a download, JSON, a text file) is sent there as a
	// navigation: htmx would put it in the page's body.
	if c.request.Method == http.MethodGet && c.request.Header.Get("HX-Boosted") == "true" {
		if kind := header.Get("Content-Type"); kind != "" && !strings.HasPrefix(kind, "text/html") && header.Get("HX-Redirect") == "" && header.Get("HX-Location") == "" {
			header.Set("HX-Redirect", c.request.URL.RequestURI())
			header.Del("Content-Length")
			c.sent = true
		}
	}
	if c.request.Header.Get("X-Go-Runtime") != "" {
		if who := c.served(); len(who) > 0 {
			header.Set(ServedHeader, strings.Join(who, ", "))
		}
	}
	written := c.written()
	if len(written) == 0 {
		return
	}
	// Never older than what the request brought: of two answers that cross, neither sets the reader back.
	value := written.Join(brought(c.request)).String()
	header.Set(cloud.BookmarkHeader, value)
	secure := c.request.TLS != nil || c.request.URL.Scheme == "https" || c.request.Header.Get("X-Forwarded-Proto") == "https"
	// Not for a script to read, and sent with a link followed here from elsewhere (Lax), which a redirect after a
	// form is.
	http.SetCookie(c, &http.Cookie{Name: cloud.BookmarkCookie, Value: value, Path: "/", MaxAge: cloud.BookmarkLasts,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func (c *carried) WriteHeader(status int) {
	c.give()
	c.ResponseWriter.WriteHeader(status)
}

func (c *carried) Write(p []byte) (int, error) {
	c.give()
	if c.sent {
		return len(p), nil // the reader is being sent to it as a navigation: its body goes nowhere
	}
	return c.ResponseWriter.Write(p)
}

// Flush sends what was written, for a handler that streams.
func (c *carried) Flush() {
	c.give()
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap is the writer underneath, for http.ResponseController.
func (c *carried) Unwrap() http.ResponseWriter { return c.ResponseWriter }
