//go:build js && wasm

package bindings

import "net/http"

// Background registers what the Worker does that is no request: the Cron Trigger's task, the queue's consumer and
// the consumer of email. main calls it before it serves.
func Background() {
	onCron()
	onQueue()
	onEmail()
}

// Read uses every binding once and gives what the page shows. A binding that fails shows its error on its own card;
// only a missing KV namespace or bucket, which most cards need, fails the page.
func Read(r *http.Request) (State, error) {
	ns, err := namespace()
	if err != nil {
		return State{}, err
	}
	s := State{OnWorkers: true}
	s.Name = variable()
	s.Visits = visit(ns)
	if s.Note, err = note(); err != nil {
		return State{}, err
	}
	s.Queued = queued(ns)
	s.Cron = get(ns, "cron")
	s.Sent, s.Received = get(ns, "sent"), get(ns, "received")
	s.Rows = rows(r.Context())
	s.Cached = cached(r)
	s.Object = object(r.Context())
	s.Socket = firstLine(r.Context(), socketHost)
	s.After = afterAnswer(ns)
	s.Request = properties(r.Context())
	s.Fetched = fetched()
	return s, nil
}

// Post is the page's forms: /bindings/note, /bindings/email and /bindings/queue.
func Post(r *http.Request) error {
	switch r.URL.Path {
	case "/bindings/note":
		return keepNote(r.PostFormValue("note"))
	case "/bindings/email":
		return send(r.PostFormValue("to"))
	case "/bindings/queue":
		return enqueue(r.PostFormValue("message"))
	}
	return http.ErrNotSupported
}
