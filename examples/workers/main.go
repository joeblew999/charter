//go:build js && wasm

// Everything workers-go has, used the workers-go way (github.com/syumai/workers-go): its own packages, called directly,
// with no charter in the way: charter builds it and wrangler runs it. One route per binding, and test/ asks each of them on wrangler
// dev (go tool charter test).
//
//	KV, R2, Queues (a producer and a consumer), D1, the Cache API, a Durable Object stub, fetch, TCP sockets,
//	variables, waitUntil, a Cron Trigger, and what Cloudflare knows of a request (request.cf)
//
// And email, sent and received, which workers-go does not have yet: charter/email, written in the same style.
//
// A file that imports workers-go's packages builds only for a Worker (js && wasm): they are syscall/js. Code that
// should also run in go test goes through charter's stand-ins instead (charter/d1, charter/bindings), as the other
// examples do.
package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/syumai/workers-go"
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
)

func main() {
	// A Cron Trigger (wrangler.jsonc's triggers): the task notes in KV which schedule ran.
	cron.ScheduleTaskNonBlock(func(ctx context.Context) error {
		event, err := cron.NewEvent(ctx)
		if err != nil {
			return err
		}
		ns, err := kv.NewNamespace("KV")
		if err != nil {
			return err
		}
		return ns.PutString("cron", event.Cron, nil)
	})
	// A queue's consumer: each message is noted in KV, and acknowledged.
	queues.ConsumeNonBlock(func(batch *queues.MessageBatch) error {
		ns, err := kv.NewNamespace("KV")
		if err != nil {
			return err
		}
		for _, m := range batch.Messages {
			body, err := m.StringBody()
			if err != nil {
				return err
			}
			if err := ns.PutString("queue:"+body, "consumed", nil); err != nil {
				return err
			}
			m.Ack()
		}
		return nil
	})

	// Email, which workers-go does not have: charter/email, in workers-go's style. A message Email Routing hands the
	// Worker is noted in KV by its sender.
	email.ConsumeNonBlock(func(_ context.Context, m *email.Message) error {
		ns, err := kv.NewNamespace("KV")
		if err != nil {
			return err
		}
		return ns.PutString("email:"+m.From, m.Header("subject"), nil)
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok") })

	// Variables: wrangler.jsonc's vars.
	mux.HandleFunc("GET /env", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, cloudflare.Getenv("APP_NAME")) })

	// KV.
	mux.HandleFunc("PUT /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		ns, err := kv.NewNamespace("KV")
		if failed(w, err) {
			return
		}
		body, _ := io.ReadAll(r.Body)
		failed(w, ns.PutString(r.PathValue("key"), string(body), nil))
	})
	mux.HandleFunc("GET /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		ns, err := kv.NewNamespace("KV")
		if failed(w, err) {
			return
		}
		value, err := ns.GetString(r.PathValue("key"), nil)
		if failed(w, err) {
			return
		}
		io.WriteString(w, value)
	})

	// R2.
	mux.HandleFunc("PUT /r2/{key}", func(w http.ResponseWriter, r *http.Request) {
		bucket, err := r2.NewBucket("BUCKET")
		if failed(w, err) {
			return
		}
		_, err = bucket.Put(r.PathValue("key"), r.Body, &r2.PutOptions{HTTPMetadata: r2.HTTPMetadata{ContentType: r.Header.Get("Content-Type")}})
		failed(w, err)
	})
	mux.HandleFunc("GET /r2/{key}", func(w http.ResponseWriter, r *http.Request) {
		bucket, err := r2.NewBucket("BUCKET")
		if failed(w, err) {
			return
		}
		object, err := bucket.Get(r.PathValue("key"))
		if failed(w, err) {
			return
		}
		if object == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", object.HTTPMetadata.ContentType)
		io.Copy(w, object.Body)
	})

	// Queues: the producer. The consumer, above, notes the message in KV.
	mux.HandleFunc("POST /queue/{message}", func(w http.ResponseWriter, r *http.Request) {
		producer, err := queues.NewProducer("QUEUE")
		if failed(w, err) {
			return
		}
		failed(w, producer.SendText(r.PathValue("message")))
	})

	// D1, through workers-go's database/sql driver.
	mux.HandleFunc("POST /d1/{name}", func(w http.ResponseWriter, r *http.Request) {
		connector, err := d1.OpenConnector("DB")
		if failed(w, err) {
			return
		}
		db := sql.OpenDB(connector)
		defer db.Close()
		if _, err := db.ExecContext(r.Context(), "INSERT INTO visitor (name) VALUES (?)", r.PathValue("name")); failed(w, err) {
			return
		}
		var n int
		if failed(w, db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM visitor").Scan(&n)) {
			return
		}
		fmt.Fprint(w, n)
	})

	// The Cache API: the first answer is stored, the next ones come from the cache.
	mux.HandleFunc("GET /cache", func(w http.ResponseWriter, r *http.Request) {
		c := cache.New()
		if hit, err := c.Match(r, nil); err == nil {
			w.Header().Set("X-Cache", "hit")
			io.Copy(w, hit.Body)
			return
		}
		answer := &http.Response{StatusCode: 200, Header: http.Header{"Cache-Control": {"max-age=60"}, "Content-Type": {"text/plain"}}, Body: io.NopCloser(strings.NewReader("cached"))}
		if failed(w, c.Put(r, answer)) {
			return
		}
		w.Header().Set("X-Cache", "miss")
		io.WriteString(w, "cached")
	})

	// A Durable Object, called through its stub (the class is charter's hub: what it answers a plain request with
	// is the proof the call arrived).
	mux.HandleFunc("GET /do", func(w http.ResponseWriter, r *http.Request) {
		namespace, err := cloudflare.NewDurableObjectNamespace("HUB")
		if failed(w, err) {
			return
		}
		stub, err := namespace.Get(namespace.IdFromName("example"))
		if failed(w, err) {
			return
		}
		req, _ := http.NewRequestWithContext(r.Context(), "GET", "https://hub/nothing", nil)
		res, err := stub.Fetch(req)
		if failed(w, err) {
			return
		}
		fmt.Fprint(w, res.StatusCode)
	})

	// fetch: a request this Worker makes.
	mux.HandleFunc("GET /fetch", func(w http.ResponseWriter, r *http.Request) {
		res, err := fetch.NewClient().HTTPClient(fetch.RedirectModeFollow).Get(r.URL.Query().Get("url"))
		if failed(w, err) {
			return
		}
		defer res.Body.Close()
		io.Copy(w, res.Body)
	})

	// A TCP socket: HTTP written by hand to host:port, and the first line of the answer.
	mux.HandleFunc("GET /socket", func(w http.ResponseWriter, r *http.Request) {
		addr := r.URL.Query().Get("addr")
		conn, err := sockets.Connect(r.Context(), addr, nil)
		if failed(w, err) {
			return
		}
		defer conn.Close()
		host, _, _ := strings.Cut(addr, ":")
		fmt.Fprintf(conn, "GET / HTTP/1.0\r\nHost: %s\r\n\r\n", host)
		line, err := bufio.NewReader(conn).ReadString('\n')
		if failed(w, err) {
			return
		}
		io.WriteString(w, line)
	})

	// Email: a message sent through the send_email binding.
	mux.HandleFunc("POST /email/{to}", func(w http.ResponseWriter, r *http.Request) {
		to := r.PathValue("to")
		raw := "From: app@example.com\r\nTo: " + to + "\r\nSubject: Hello from a Go Worker\r\nMessage-ID: <hello@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nSent with charter/email.\r\n"
		failed(w, email.Send("MAIL", "app@example.com", to, []byte(raw)))
	})

	// waitUntil: work that goes on after the answer.
	mux.HandleFunc("POST /wait-until", func(w http.ResponseWriter, _ *http.Request) {
		cloudflare.WaitUntil(func() {
			if ns, err := kv.NewNamespace("KV"); err == nil {
				ns.PutString("wait-until", "done", nil)
			}
		})
		io.WriteString(w, "answered")
	})

	// What Cloudflare knows of the request (request.cf).
	mux.HandleFunc("GET /cf", func(w http.ResponseWriter, r *http.Request) {
		properties, err := fetch.NewIncomingProperties(r.Context())
		if failed(w, err) {
			return
		}
		fmt.Fprintf(w, "%s %s", properties.Country, properties.Timezone)
	})

	// workers-go's own way to serve, in its non-blocking form: the program stays, and charter's build keeps this Go
	// runtime for the next request. (workers.Serve(mux) works too: the program then ends with each answer, as
	// workers-go's own entry expects, and every request starts a new runtime.)
	workers.ServeNonBlock(mux)
	workers.Ready()
	select {}
}

// failed answers 500 with err, and says whether there was one.
func failed(w http.ResponseWriter, err error) bool {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return err != nil
}
