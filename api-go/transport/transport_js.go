//go:build js && wasm

package transport

import (
	"context"
	"net/http"

	"github.com/syumai/workers-go"
)

// Serve cancels the request's context when the client has gone. workers-go runs one request per Go
// runtime and closes workers.Done() when the response body is closed or cancelled. The WebSocket
// adapter is JavaScript here: worker/websocket.mjs.
func Serve(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		go func() {
			select {
			case <-workers.Done():
				cancel()
			case <-ctx.Done():
			}
		}()
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
