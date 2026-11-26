// Package cloud is everything Cloudflare gives the Worker, one file each, used through workers-go's own packages
// (github.com/syumai/workers-go) by the names wrangler.jsonc gives the bindings: KV, BUCKET, QUEUE, DB, HUB, MAIL.
// The page at /bindings shows a card for each (pages/bindings.gsx), and test/cloud_test.go asks for each.
//
//	kv.go       KV: a counter                    cache.go    the Cache API
//	r2.go       R2: a note                       object.go   a Durable Object's stub
//	queue.go    Queues: producer and consumer    socket.go   a TCP socket
//	cron.go     a Cron Trigger                   wait.go     waitUntil
//	d1.go       D1 through database/sql          request.go  request.cf
//	fetch.go    fetch                            vars.go     a variable
//	email.go    email, out and in (charter/bindings: workers-go has none yet)
//
// Those files build only for a Worker, as workers-go does. Off one (go test, go run .) the page renders and says so
// (other.go).
package bindings

import (
	"strings"
	"time"
)

// State is what the page shows.
type State struct {
	OnWorkers bool     // false off Workers (go test)
	Visits    int      // KV: how often the page was opened
	Note      string   // R2: the note kept in the bucket
	Queued    []string // Queues: the messages the consumer has taken, newest first
	Cron      string   // Cron Triggers: the schedule that last ran, and when
	Name      string   // a variable of the Worker
	Fetched   string   // fetch: what another site answered
	Rows      string   // D1: how many messages the database has
	Cached    string   // the Cache API: whether the value came from the cache, and when it was stored
	Object    string   // a Durable Object: what its stub answered
	Socket    string   // a TCP socket: the first line the other end sent
	After     string   // waitUntil: when the work after the last answer finished
	Request   string   // the request's properties (request.cf)
	Sent      string   // email: whom the last message was sent to
	Received  string   // email: the sender and subject of the last message received
}

// When each of these happened, for the page to show in the reader's language and time zone (intl's date filter):
// the time is the RFC 3339 part of what was kept.

func (s State) CronAt() time.Time  { return at(s.Cron) }
func (s State) AfterAt() time.Time { return at(s.After) }
func (s State) SentAt() time.Time  { return at(s.Sent) }

// CronSchedule is the schedule that ran, without when; SentTo is whom the message went to, shortened.
func (s State) CronSchedule() string { return strings.TrimSpace(without(s.Cron)) }
func (s State) SentTo() string       { return strings.TrimSpace(without(s.Sent)) }

func at(kept string) time.Time {
	for _, word := range strings.Fields(kept) {
		if t, err := time.Parse(time.RFC3339Nano, word); err == nil {
			return t
		}
	}
	return time.Time{}
}

func without(kept string) string {
	var words []string
	for _, word := range strings.Fields(kept) {
		if _, err := time.Parse(time.RFC3339Nano, word); err != nil {
			words = append(words, word)
		}
	}
	return strings.Join(words, " ")
}
