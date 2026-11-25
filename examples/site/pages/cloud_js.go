//go:build js && wasm

package pages

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/cron"
	"github.com/syumai/workers-go/cloudflare/fetch"
	"github.com/syumai/workers-go/cloudflare/kv"
	"github.com/syumai/workers-go/cloudflare/queues"
	"github.com/syumai/workers-go/cloudflare/r2"

	"github.com/joeblew999/charter/email"
	"github.com/joeblew999/charter/transport"
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
		log, _ := ns.GetString("queued", nil)
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
		n, _ := ns.GetString("visits", nil)
		s.Visits, _ = strconv.Atoi(n)
		s.Visits++
		if err := ns.PutString("visits", strconv.Itoa(s.Visits), nil); err != nil {
			fail(w, err)
			return
		}
		if object, err := bucket.Get("note.txt"); err == nil && object != nil {
			b, _ := io.ReadAll(object.Body)
			s.Note = string(b)
		}
		if log, _ := ns.GetString("queued", nil); log != "" {
			s.Queued = strings.Split(log, "\n")
		}
		s.Cron, _ = ns.GetString("cron", nil)
		s.Sent, _ = ns.GetString("sent", nil)
		s.Received, _ = ns.GetString("received", nil)
		// What Cloudflare knows of the request (request.cf), as charter's transport carries it.
		cf := transport.CFOf(r)
		s.Place = strings.TrimSpace(cf.Country + " " + cf.Timezone)
		// fetch: a request the Worker makes, here to its own health check.
		if res, err := fetch.NewClient().HTTPClient(fetch.RedirectModeFollow).Get(web.Origin(r) + "/healthz"); err == nil {
			b, _ := io.ReadAll(res.Body)
			res.Body.Close()
			s.Fetched = "GET /healthz → " + strconv.Itoa(res.StatusCode) + " " + string(b)
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
