//go:build js && wasm

package transport

import (
	"context"
	"net/http"
	"runtime"
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

// Run serves h on Cloudflare and never returns, so the Go runtime stays alive after a response and
// worker/go.mjs can give it the next request. (workers.Serve returns when the first response's body
// is closed, and the program ends.)
//
// A runtime that go.mjs starts while the Worker's module loads is given paths to warm up with (the
// binding's "warm"): Run then answers a GET of each before any request comes, into nothing. A path
// must be one whose handler touches no binding, like the OpenAPI route, which also registers every
// operation. What a new isolate's first request would have paid for is then done already.
func Run(h http.Handler) {
	if paths := js.Global().Get("context").Get("binding").Get("warm"); paths.Type() == js.TypeObject {
		for i := 0; i < paths.Length(); i++ {
			if request, err := http.NewRequest(http.MethodGet, "https://warm.invalid"+paths.Index(i).String(), nil); err == nil {
				h.ServeHTTP(&discard{header: http.Header{}}, request)
			}
		}
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	allocated = stats.TotalAlloc // start-up is not a request's allocation
	workers.ServeNonBlock(Serve(h))
	workers.Ready()
	select {}
}

// discard is a ResponseWriter that keeps nothing.
type discard struct{ header http.Header }

func (d *discard) Header() http.Header         { return d.header }
func (d *discard) Write(p []byte) (int, error) { return len(p), nil }
func (d *discard) WriteHeader(int)             {}

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
	binding := js.Global().Get("context").Get("binding")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		gone := js.FuncOf(func(js.Value, []js.Value) any {
			cancel()
			return nil
		})
		binding.Set("cancel", gone)
		defer func() {
			binding.Set("cancel", js.Undefined())
			gone.Release()
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

		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if request := stats.TotalAlloc - allocated; request > mostRequest {
			mostRequest = request
		}
		allocated = stats.TotalAlloc
		// Twice the largest request so far: what follows this handler (the body being read) allocates too.
		if stats.NumGC > 0 || stats.HeapIdle < 2*mostRequest {
			binding.Set("full", true)
		}
	})
}
