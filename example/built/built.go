// Package built is what the app does with what Cloudflare gives a Worker: each thing does one job in the notes app,
// and has a page of its own that shows it (pages: the group Built). Nothing is used before the page that needs it is
// opened, or the note that needs it is written.
//
//	attachments.go   R2: a file attached to a note        notice.go    KV: a line for everyone, on every visit
//	activity.go      Queues, a Cron Trigger and D1: what happened, followed live through a Durable Object
//	mail.go          email, out and in                    limits.go    a rate limit, and the Cache API
//	outside.go       fetch and a TCP socket, to another site, when asked
//
// Those build only for a Worker, with workers-go's own packages (github.com/syumai/workers-go), by the names
// wrangler.jsonc gives the bindings: BUCKET, KV, QUEUE, DB, HUB, MAIL, WRITE_LIMIT. Off one (a test of a page) each
// gives nothing and says the Worker is needed (other.go).
package built

import (
	"net/http"
	"time"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/db"
	"github.com/joeblew999/charter/example/env"
	"github.com/joeblew999/charter/live"
)

// What every time and number here is kept as: a time, a count. The pages show them through Intl, in the reader's
// language and time zone; nothing is kept as text that was formatted once.

// Activity is something that happened: a note written (Kind "note": its writer, its id), or a day's count
// (Kind "summary": Notes).
type Activity struct{ db.ActivityRow }

// The kinds of Activity.
const (
	NoteWritten = "note"
	Summary     = "summary"
)

// At is when it happened: D1's CURRENT_TIMESTAMP, which is UTC.
func (a Activity) At() time.Time { return stamp(a.CreatedAt) }

// Position is its place in the log, which its feed follows.
func (a Activity) Position() int64 { return a.ID }

// Feed is the activity, followed live: the table is the log, the Durable Object HUB wakes whoever has the page open.
var Feed = live.Feed[Activity]{Binding: env.HUB, Name: "activity", Log: func() (live.Log[Activity], error) { return openLog() }}

// Attachment is a file someone attached: what the database knows of it. The file itself is in the bucket.
type Attachment struct{ db.AttachmentsRow }

func (a Attachment) At() time.Time { return stamp(a.CreatedAt) }

// MaxFile is the largest file that may be attached.
const MaxFile = 1 << 20

// Notice is a line for everyone, which whoever may moderate writes: shown on the notes page to every reader.
type Notice struct {
	Text string
	Set  time.Time
}

// MaxNotice is the longest notice.
const MaxNotice = 200

// Mail is the last message the Worker sent and the last it was handed. Addresses are kept shortened (a…@example.com):
// the page is public.
type Mail struct {
	SentTo           string
	Sent             time.Time
	From, Subject    string
	Received         time.Time
	MayNotSendBefore time.Time // one message an hour: when the next may go
}

// Report is the notes in numbers, which takes a query to work out: kept in the cache for a minute.
type Report struct {
	Notes, Authors int64
	At             time.Time     // when it was worked out
	FromCache      bool          // whether this answer came from the cache
	Took           time.Duration // how long this answer took
}

// Limit is the write limit, and whether a call just made was within it.
type Limit struct {
	Calls   int // how many, in Seconds
	Seconds int
	Asked   bool // whether a call was just made
	Allowed bool
}

// Outside is what another site answered, and how long it took.
type Outside struct {
	Asked  string
	Said   string
	Took   time.Duration
	Failed bool
}

// Facts is what Cloudflare says of the request, and the Worker's own name: no binding is asked for any of it.
type Facts struct {
	Name, Country, City, Timezone, Colo string
}

// FactsOf is the request's.
func FactsOf(r *http.Request) Facts {
	cf := app.CFOf(r)
	return Facts{Name: env.APP_NAME.Value(), Country: cf.Country, City: cf.City, Timezone: cf.Timezone, Colo: cf.Colo}
}

// stamp is a time as D1's CURRENT_TIMESTAMP wrote it.
func stamp(s string) time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	return t
}

// MayDetach is the rule for taking a file away, as api.MayRemove is for a note: whoever attached it, and whoever may
// moderate.
func MayDetach(who charter.Caller, a Attachment) bool {
	return who.May(api.Moderate) || (who.Subject != "" && who.Subject == a.AuthorID)
}
