package cloud

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// Reads from the nearest replica, and a reader who always sees their own writes.
//
// D1 keeps a copy of a database in each of its regions once read replication is on, and answers a read from one of
// them only for a session (https://developers.cloudflare.com/d1/best-practices/read-replication/): without one,
// every statement goes to the one primary. A copy may be behind. So the statements of one request share one
// session, which makes them agree with each other, and a session can start from a bookmark, a point in the
// database's history, which makes it at least as new as that.
//
// charter gives every request its session, with nothing for an app to write: app.Serve begins it and ends it, and
// Open hands it to whatever opens the database. Where a request's session starts:
//
//   - at the primary, when the request says so with Newest: a caller is being identified (a session or a token
//     that was just taken away must not be accepted by a copy that is behind), or a reader is fetching what a feed
//     just told them of;
//   - else at the bookmark the request brought, which a response gave this reader after their own write (a cookie
//     for a browser, so it survives a redirect; a header for a program);
//   - else anywhere: the nearest copy.
//
// A statement outside a request (a Cron Trigger, a queue's batch, an email, work left for after the answer) has no
// session to share: each starts at the primary.
//
// This is what workers-go lacks, and so charter's: its d1 package (v0.36.0, a database/sql driver) calls the binding
// itself and knows no session and no bookmark, so SQL, which is that driver, reads the primary always. When
// workers-go has sessions, this is to be built on them.
//
// The idea and its cases are Void's (voidzero-dev/void, runtime/db/session.ts), which leaves the carrying to each
// app; here it is done once.

// How a session starts when no bookmark says where: D1's own two words.
const (
	fromPrimary  = "first-primary"
	fromAnywhere = "first-unconstrained"
)

// BookmarkCookie and BookmarkHeader are where a response gives a reader the bookmark after their write, and where
// their next request brings it back: the cookie for a browser, the header for a program. Each holds Bookmarks'
// String.
const (
	BookmarkCookie = "d1"
	BookmarkHeader = "X-D1-Bookmark"
)

// BookmarkLasts is how long a browser keeps the cookie, in seconds. A copy is behind by less than a second as a
// rule; the cookie outlives that by far, and is gone before it is one more thing every request carries.
const BookmarkLasts = 300

// Bookmarks is a bookmark for each database, by its binding's name.
type Bookmarks map[string]string

// A bookmark is opaque, and has a shape: 0000007b-0000b26e-00001538-0c3e87bb37b3db5cc52eedb93cd3b96b. One that has
// not is no bookmark of D1's, and is never given to it.
var bookmarkShape = regexp.MustCompile(`^[0-9a-f]{8}(-[0-9a-f]{8}){2}-[0-9a-f]{32}$`)

// ParseBookmarks reads what a request brought (a cookie's value, a header's). What is not a bookmark is left out,
// so a value that is malformed, or someone's invention, starts nothing.
func ParseBookmarks(value string) Bookmarks {
	found := Bookmarks{}
	pairs, err := url.ParseQuery(value)
	if err != nil {
		return found
	}
	for name, values := range pairs {
		if len(values) == 1 && name != "" && bookmarkShape.MatchString(values[0]) {
			found[name] = values[0]
		}
	}
	return found
}

// String is the bookmarks as a cookie's value or a header's: DB=0000007b-…, the databases in order.
func (b Bookmarks) String() string {
	names := make([]string, 0, len(b))
	for name := range b {
		names = append(names, name)
	}
	slices.Sort(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = url.QueryEscape(name) + "=" + b[name]
	}
	return strings.Join(parts, "&")
}

// Newer is the later of two bookmarks of one database: they sort as text, oldest first
// (https://developers.cloudflare.com/d1/reference/time-travel/#bookmarks). The empty one is none.
func Newer(a, b string) string {
	if a > b {
		return a
	}
	return b
}

// Join is b with each bookmark of other that is newer, or that b has not: what a response gives a reader is never
// older than what their request brought, so two responses that cross cannot set a reader back.
func (b Bookmarks) Join(other Bookmarks) Bookmarks {
	joined := Bookmarks{}
	for name, bookmark := range b {
		joined[name] = bookmark
	}
	for name, bookmark := range other {
		joined[name] = Newer(joined[name], bookmark)
	}
	return joined
}

// startOf is where a request's session on a database starts: the primary when the request reads the newest, the
// bookmark it brought, or anywhere.
func startOf(newest bool, brought string) string {
	switch {
	case newest:
		return fromPrimary
	case brought != "":
		return brought
	}
	return fromAnywhere
}
