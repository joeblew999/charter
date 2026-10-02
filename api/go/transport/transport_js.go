//go:build js && wasm

package transport

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"runtime"
	"strings"
	"syscall/js"

	"github.com/syumai/workers-go"
)

// What this Go runtime has served: how many requests, the allocation counter after the last one,
// and the most one request allocated.
var (
	served      int
	allocated   uint64
	mostRequest uint64
)

// What worker/go.mjs and this runtime call each other through, and JavaScript's byte array.
var (
	binding    = js.Global().Get("context").Get("binding")
	uint8Array = js.Global().Get("Uint8Array")
)

// Run serves h on Cloudflare and never returns, so the Go runtime stays alive after a response and
// worker/go.mjs can give it the next request. (workers.Serve returns when the first response's body
// is closed, and the program ends.)
//
// A runtime that go.mjs starts while the Worker's module loads is given paths to warm up with (the
// binding's "warm"): Run then answers a GET of each before any request comes, into nothing. A path
// must be one whose handler touches no binding, like the OpenAPI route, which also registers every
// operation. What a new isolate's first request would have paid for is then done already.
//
// Requests come through the binding's "serve" (answer, below), not through workers-go's
// handleRequest, which costs a hello a dozen goroutines and some hundred values crossing between
// JavaScript and Go. workers-go still gets the handler, so its own entry (build/worker.mjs) works
// too.
func Run(h http.Handler) {
	if paths := binding.Get("warm"); paths.Type() == js.TypeObject {
		for i := 0; i < paths.Length(); i++ {
			if request, err := http.NewRequest(http.MethodGet, "https://warm.invalid"+paths.Index(i).String(), nil); err == nil {
				h.ServeHTTP(&discard{header: http.Header{}}, request)
			}
		}
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	allocated = stats.TotalAlloc // start-up is not a request's allocation
	handler := Serve(h)
	binding.Set("serve", js.FuncOf(func(_ js.Value, args []js.Value) any {
		answer(handler, args[0].String(), args[1])
		return nil
	}))
	workers.ServeNonBlock(handler)
	workers.Ready()
	select {}
}

// discard is a ResponseWriter that keeps nothing.
type discard struct{ header http.Header }

func (d *discard) Header() http.Header         { return d.header }
func (d *discard) Write(p []byte) (int, error) { return len(p), nil }
func (d *discard) WriteHeader(int)             {}

// answer runs one request, in the goroutine of the call from JavaScript. Every value that crosses
// between JavaScript and Go costs, so worker/go.mjs gives the request as two: head is the method,
// the URL, and then each header's name and its value, one per line; body is the bytes (a
// Uint8Array), or null.
//
// The answer goes back through two functions go.mjs puts on the binding:
//
//   - respond(status, head, body, more): the status, the headers (a name and its value, one per
//     line), and what was written so far. A handler that returns without flushing, as every JSON
//     answer does, costs this one call, with more false.
//   - write(body, more): what was written since, once the answer is a stream. It returns false
//     when the client has gone.
//
// An answer becomes a stream at the handler's first Flush (http.Flusher), or when streamLimit
// bytes are waiting. Nothing else sends bytes before the handler returns, as with net/http.
func answer(h http.Handler, head string, body js.Value) {
	lines := strings.Split(head, "\n")
	w := &response{header: http.Header{}}
	r, err := http.NewRequest(lines[0], lines[1], http.NoBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		w.send(false)
		return
	}
	for i := 2; i+1 < len(lines); i += 2 {
		r.Header.Add(lines[i], lines[i+1])
	}
	r.RemoteAddr = r.Header.Get("Cf-Connecting-Ip")
	if !body.IsNull() {
		r.Body, r.ContentLength = &requestBody{js: body}, int64(body.Length())
	}
	h.ServeHTTP(w, r)
	w.send(false)
}

// requestBody copies the request's body out of JavaScript when the handler first reads it.
type requestBody struct {
	js     js.Value
	copied bool
	data   bytes.Reader
}

func (b *requestBody) Read(p []byte) (int, error) {
	if !b.copied {
		data := make([]byte, b.js.Length())
		js.CopyBytesToGo(data, b.js)
		b.data.Reset(data)
		b.copied = true
	}
	return b.data.Read(p)
}

func (b *requestBody) Close() error { return nil }

// streamLimit is how much of an answer waits in Go before it is sent on as a stream.
const streamLimit = 1 << 20

// response is the ResponseWriter of answer: it keeps what the handler writes until send.
type response struct {
	header    http.Header
	status    int
	body      []byte
	streaming bool // the status and headers have gone to JavaScript
	gone      bool // and the client has gone
}

func (w *response) Header() http.Header { return w.header }

func (w *response) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *response) Write(p []byte) (int, error) {
	if w.gone {
		return 0, io.ErrClosedPipe
	}
	w.WriteHeader(http.StatusOK)
	if w.body = append(w.body, p...); len(w.body) > streamLimit {
		w.send(true)
	}
	return len(p), nil
}

// Flush sends what was written so far: the answer is a stream from here on.
func (w *response) Flush() { w.send(true) }

// send gives JavaScript what was written since the last send. more says whether the handler is
// still running.
func (w *response) send(more bool) {
	body := js.Null()
	if len(w.body) > 0 {
		body = uint8Array.New(len(w.body))
		js.CopyBytesToJS(body, w.body)
		w.body = w.body[:0]
	}
	if w.streaming {
		w.gone = !binding.Call("write", body, more).Bool()
		return
	}
	w.streaming = true
	w.WriteHeader(http.StatusOK)
	var head []byte
	for name, values := range w.header {
		for _, value := range values {
			head = append(append(append(append(head, name...), '\n'), newlines.Replace(value)...), '\n')
		}
	}
	binding.Call("respond", w.status, string(head), body, more)
}

// A header's value is one line: a newline in it would be read as the next header.
var newlines = strings.NewReplacer("\n", " ", "\r", " ")

// Serve cancels the request's context when the client has gone, and says when this Go runtime
// should not get another request.
//
// The client has gone: the Worker's entry says so. worker/go.mjs calls the binding's cancel
// function, which is this request's while it runs (a runtime serves one request at a time). Under
// workers-go's own entry, which runs one request per runtime, workers.Done() says it: it is closed
// when the response body is closed or cancelled.
//
// No more requests: a runtime is reused only while its heap has room for another request without
// the collector running. A collection in a full heap cost 65 to 293 ms of CPU on Cloudflare, far
// more than a new runtime does, and TinyGo's collector got little back. When the room is gone the
// binding's "full" is set, and worker/go.mjs drops the runtime after this response.
//
// The WebSocket adapter is JavaScript here: worker/websocket.mjs.
func Serve(h http.Handler) http.Handler {
	// One function for the life of the runtime: it cancels whichever request is running.
	var cancelRequest context.CancelFunc
	binding.Set("cancel", js.FuncOf(func(js.Value, []js.Value) any {
		if cancelRequest != nil {
			cancelRequest()
		}
		return nil
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		cancelRequest = cancel
		defer func() {
			cancelRequest = nil
			cancel()
		}()
		if served++; served == 1 {
			go func() {
				select {
				case <-workers.Done():
					cancel()
				case <-ctx.Done():
				}
			}()
		}
		h.ServeHTTP(w, r.WithContext(ctx))

		// Reading the heap's state walks its whole block table, so it is done for the first request
		// and then for one in eight, with room kept for the eight in between.
		if served%8 != 1 {
			return
		}
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if since := stats.TotalAlloc - allocated; since > mostRequest {
			mostRequest = since
		}
		allocated = stats.TotalAlloc
		// Twice what was allocated since the last look, or by the first request alone.
		if stats.NumGC > 0 || stats.HeapIdle < 2*mostRequest {
			binding.Set("full", true)
		}
	})
}
