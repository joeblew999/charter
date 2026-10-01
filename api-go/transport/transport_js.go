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
func Run(h http.Handler) {
	workers.ServeNonBlock(Serve(h))
	workers.Ready()
	select {}
}

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
