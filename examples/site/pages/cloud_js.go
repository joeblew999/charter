//go:build js && wasm

package pages

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/cache"
	"github.com/syumai/workers-go/cloudflare/cron"
	"github.com/syumai/workers-go/cloudflare/d1"
	"github.com/syumai/workers-go/cloudflare/fetch"
	"github.com/syumai/workers-go/cloudflare/kv"
	"github.com/syumai/workers-go/cloudflare/queues"
	"github.com/syumai/workers-go/cloudflare/r2"
	"github.com/syumai/workers-go/cloudflare/sockets"

	"github.com/joeblew999/charter/email"
	"github.com/joeblew999/charter/web"
)

// The Cloudflare page's bindings, each through workers-go's own package (github.com/syumai/workers-go), by the names
// wrangler.jsonc gives them: KV, BUCKET, QUEUE. This file builds only for a Worker, as workers-go does.

// Background registers what the Worker does that is no request: a Cron Trigger's task and a queue's consumer. main
// calls it before it serves.
func Background() {
	// The Cron Trigger: which schedule ran, and when.
	cron.ScheduleTaskNonBlock(func(ctx context.Context) error {
		event, err := cron.NewEvent(ctx)
		if err != nil {
			return err
		}
		ns, err := kv.NewNamespace("KV")
		if err != nil {
			return err
		}
		return ns.PutString("cron", event.Cron+"  "+time.Now().UTC().Format(time.RFC3339), nil)
	})
	// Email in (charter/email: workers-go has none yet): who wrote, and the subject.
	email.ConsumeNonBlock(func(_ context.Context, m *email.Message) error {
		ns, err := kv.NewNamespace("KV")
		if err != nil {
			return err
		}
		return ns.PutString("received", m.From+": "+m.Header("subject"), nil)
	})
	// The queue's consumer: each message goes to the top of a short list in KV.
	queues.ConsumeNonBlock(func(batch *queues.MessageBatch) error {
		ns, err := kv.NewNamespace("KV")
		if err != nil {
			return err
		}
		log := value(ns, "queued")
		lines := strings.Split(log, "\n")
		for _, m := range batch.Messages {
			body, err := m.StringBody()
			if err != nil {
				return err
			}
			lines = append([]string{body}, lines...)
			m.Ack()
		}
		if len(lines) > 5 {
			lines = lines[:5]
		}
		return ns.PutString("queued", strings.TrimSpace(strings.Join(lines, "\n")), nil)
	})
}

// elsewhere is a site that is not this one, for the two cards that reach out: IANA's example domain.
const elsewhere = "example.com"

// socketHost is where the TCP socket card connects: a site that is not on Cloudflare's own network, which a Worker's
// sockets may not reach (nor may they reach the Worker's own address).
const socketHost = "www.google.com"

// firstLine opens a TCP socket to host's port 80, asks for its front page by hand, and gives the first line of the
// answer, or what went wrong. workers-go's socket panics when a connection Cloudflare refused is closed, which would
// end this Go runtime: that is caught here and shown instead.
func firstLine(ctx context.Context, host string) (line string) {
	defer func() {
		if p := recover(); p != nil {
			line = fmt.Sprint(host, ":80: ", p)
		}
	}()
	conn, err := sockets.Connect(ctx, host+":80", nil)
	if err != nil {
		return err.Error()
	}
	defer conn.Close()
	fmt.Fprintf(conn, "HEAD / HTTP/1.0\r\nHost: %s\r\n\r\n", host)
	first, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return host + ":80: " + err.Error()
	}
	return host + ":80 → " + strings.TrimSpace(first)
}

// cloud serves /cloud and its forms.
func cloud(w http.ResponseWriter, r *http.Request) {
	ns, err := kv.NewNamespace("KV")
	if err != nil {
		fail(w, err)
		return
	}
	bucket, err := r2.NewBucket("BUCKET")
	if err != nil {
		fail(w, err)
		return
	}
	switch {
	case r.URL.Path == "/cloud/note" && r.Method == http.MethodPost:
		// R2: the note is one object.
		if _, err := bucket.Put("note.txt", io.NopCloser(strings.NewReader(r.PostFormValue("note"))), nil); err != nil {
			fail(w, err)
			return
		}
		back(w, r)
	case r.URL.Path == "/cloud/email" && r.Method == http.MethodPost:
		// Email out, through the send_email binding MAIL.
		to := r.PostFormValue("to")
		raw := "From: site@example.com\r\nTo: " + to + "\r\nSubject: Hello from charter-site\r\nMessage-ID: <" + strconv.FormatInt(time.Now().UnixNano(), 36) + "@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nSent from a Go Worker.\r\n"
		if err := email.Send("MAIL", "site@example.com", to, []byte(raw)); err != nil {
			fail(w, err)
			return
		}
		if err := ns.PutString("sent", "→ "+to, nil); err != nil {
			fail(w, err)
			return
		}
		back(w, r)
	case r.URL.Path == "/cloud/queue" && r.Method == http.MethodPost:
		// Queues: the producer. The consumer, in Background, has it a moment later.
		producer, err := queues.NewProducer("QUEUE")
		if err == nil {
			err = producer.SendText(r.PostFormValue("message"))
		}
		if err != nil {
			fail(w, err)
			return
		}
		back(w, r)
	case r.URL.Path == "/cloud" && r.Method == http.MethodGet:
		s := CloudState{OnWorkers: true, Name: cloudflare.Getenv("APP_NAME")}
		// KV: a counter.
		n := value(ns, "visits")
		s.Visits, _ = strconv.Atoi(n)
		s.Visits++
		// Cloudflare takes one write a second to a key (429 beyond it): a visit within the same second as another
		// is shown and not counted, and the page still answers.
		if err := ns.PutString("visits", strconv.Itoa(s.Visits), nil); err != nil {
			log.Printf("pages: visits not counted: %v", err)
		}
		if object, err := bucket.Get("note.txt"); err == nil && object != nil {
			b, _ := io.ReadAll(object.Body)
			s.Note = string(b)
		}
		if log := value(ns, "queued"); log != "" {
			s.Queued = strings.Split(log, "\n")
		}
		s.Cron = value(ns, "cron")
		s.Sent = value(ns, "sent")
		s.Received = value(ns, "received")
		origin := web.Origin(r)
		// D1, through workers-go's database/sql driver: how many messages the home page's list has.
		if connector, err := d1.OpenConnector("DB"); err == nil {
			db := sql.OpenDB(connector)
			var rows int
			if err := db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM messages").Scan(&rows); err == nil {
				s.Rows = strconv.Itoa(rows)
			} else {
				s.Rows = err.Error()
			}
			db.Close()
		}
		// The Cache API: a value stored under an address of this site, for a minute. The first visit stores it.
		at, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, origin+"/cloud/cached", nil)
		c := cache.New()
		if hit, err := c.Match(at, nil); err == nil {
			b, _ := io.ReadAll(hit.Body)
			s.Cached = "hit: stored " + string(b)
		} else {
			now := time.Now().UTC().Format(time.RFC3339)
			// The length is given: workers-go then writes the body to a fixed-length stream. Without it the body is a
			// stream Cloudflare pulls once more after its end; workers-go v0.36.0 closes it a second time, and that
			// error ends the Go program.
			err := c.Put(at, &http.Response{StatusCode: 200, ContentLength: int64(len(now)), Header: http.Header{"Cache-Control": {"max-age=60"}, "Content-Type": {"text/plain"}}, Body: io.NopCloser(strings.NewReader(now))})
			if err != nil {
				s.Cached = err.Error()
			} else {
				s.Cached = "miss: stored " + now
			}
		}
		// A Durable Object, through its stub: the hub that wakes the home page's stream. A plain request is not what
		// it serves, so its status is the proof the call arrived.
		if namespace, err := cloudflare.NewDurableObjectNamespace("HUB"); err == nil {
			if stub, err := namespace.Get(namespace.IdFromName("messages")); err == nil {
				ask, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://hub/", nil)
				if res, err := stub.Fetch(ask); err == nil {
					s.Object = "Hub → HTTP " + strconv.Itoa(res.StatusCode)
				} else {
					s.Object = err.Error()
				}
			}
		}
		// A TCP socket: HTTP written by hand to another site, and the first line of the answer.
		s.Socket = firstLine(r.Context(), socketHost)
		// waitUntil: work that goes on after this answer has left. The page shows when the last such work finished.
		s.After = value(ns, "after")
		cloudflare.WaitUntil(func() {
			ns.PutString("after", time.Now().UTC().Format(time.RFC3339Nano), nil)
		})
		// The request's properties (request.cf), as workers-go reads them.
		if p, err := fetch.NewIncomingProperties(r.Context()); err == nil {
			s.Request = strings.Join([]string{p.Country, p.City, p.Timezone, p.Colo, p.HttpProtocol}, " · ")
		} else {
			s.Request = err.Error()
		}
		// fetch: a request the Worker makes, to another site (Cloudflare refuses one to the Worker's own address).
		if res, err := fetch.NewClient().HTTPClient(fetch.RedirectModeFollow).Get("https://" + elsewhere + "/"); err == nil {
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			s.Fetched = "GET https://" + elsewhere + "/ → " + strconv.Itoa(res.StatusCode)
		} else {
			s.Fetched = err.Error()
		}
		render(w, r, http.StatusOK, Cloud(s))
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// back sends the browser to the page it posted from.
func back(w http.ResponseWriter, r *http.Request) {
	to := "/cloud"
	if ref := r.Referer(); ref != "" {
		if i := strings.Index(ref, "://"); i >= 0 {
			if j := strings.Index(ref[i+3:], "/"); j >= 0 && strings.HasSuffix(strings.SplitN(ref[i+3+j:], "?", 2)[0], "/cloud") {
				to = strings.SplitN(ref[i+3+j:], "?", 2)[0]
			}
		}
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// value is what KV has under key, "" when it has nothing: workers-go gives a missing key as the text "<null>".
func value(ns *kv.Namespace, key string) string {
	v, err := ns.GetString(key, nil)
	if err != nil || v == "<null>" {
		return ""
	}
	return v
}
