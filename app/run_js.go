//go:build js && wasm

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"syscall/js"

	"github.com/syumai/workers-go"
	"github.com/syumai/workers-go/cloudflare/fetch"

	"github.com/joeblew999/charter/cloud"
)

// What this Go runtime has served, for deciding when it should serve no more.
var served int

// binding is what worker/go.mjs and this runtime say to each other through.
var binding = js.Global().Get("context").Get("binding")

// gone ends the context of the request this runtime is serving (it serves one at a time): the binding's "cancel".
var gone context.CancelFunc

func init() {
	binding.Set("cancel", js.FuncOf(func(js.Value, []js.Value) any {
		if gone != nil {
			gone()
		}
		return nil
	}))
}

// Run serves h on Cloudflare the workers-go way and never returns: the handler is workers-go's
// (workers.ServeNonBlock), so a request reaches it through workers-go's own path and everything of workers-go that
// reads the request works in h (fetch.NewIncomingProperties, the Cache API). The program does not end with the
// answer, as workers.Serve's does, so worker/go.mjs can give this Go runtime the next request.
//
// A runtime that go.mjs starts while the Worker's module loads is given paths to warm up with (the binding's "warm"):
// Run then answers a GET of each before any request comes, into nothing. A path must be one whose handler touches no
// binding. What a new isolate's first request would have paid for is then done already.
func Run(h http.Handler) {
	if paths := binding.Get("warm"); paths.Type() == js.TypeObject {
		for i := 0; i < paths.Length(); i++ {
			if request, err := http.NewRequest(http.MethodGet, "https://warm.invalid"+paths.Index(i).String(), nil); err == nil {
				h.ServeHTTP(&discard{header: http.Header{}}, request)
			}
		}
	}
	workers.ServeNonBlock(Serve(h))
	workers.Ready()
	select {}
}

// discard is a ResponseWriter that keeps nothing.
type discard struct{ header http.Header }

func (d *discard) Header() http.Header         { return d.header }
func (d *discard) Write(p []byte) (int, error) { return len(p), nil }
func (d *discard) WriteHeader(int)             {}

// Serve is h with what charter adds to a request on Cloudflare: where it came from (CFOf, read from workers-go's own
// request properties), its client's address as RemoteAddr, and, after it, whether this Go runtime should get another
// request: one that holds too much memory sets the binding's "full", and worker/go.mjs lets it go.
//
// The WebSocket adapter is JavaScript here: worker/websocket.mjs.
func Serve(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RemoteAddr == "" {
			r.RemoteAddr = r.Header.Get("Cf-Connecting-Ip")
		}
		// workers-go's request context never ends. Here it ends with the handler, or before it when the reader has
		// gone away (go.mjs calls the binding's "cancel"): a handler that waits, an event stream, then returns.
		ctx, cancel := context.WithCancel(r.Context())
		gone = cancel
		defer cancel()
		// The last thing: go.mjs is told the handler has returned, and may give this runtime its next request.
		defer func() {
			if ended := binding.Get("ended"); ended.Type() == js.TypeFunction {
				ended.Invoke()
			}
		}()
		// The request's statements on D1 share a session, from the bookmark it brought; its answer gives the reader
		// the bookmark of what it wrote (bookmark.go, cloud/session.go).
		cloud.Begin(brought(r))
		defer cloud.End()
		// A part of a page that asks for itself again because its feed said something changed reads what is newest.
		if r.Header.Get(cloud.NewestHeader) != "" {
			cloud.Newest()
		}
		out := carrying(w, r, cloud.Reached, cloud.Served)
		h.ServeHTTP(out, incoming(r.WithContext(ctx)))
		out.give()

		// Reading the heap's state is not free: for the first request, then for one in eight.
		if served++; served%8 != 1 {
			return
		}
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if full(&stats, 0) {
			binding.Set("full", true)
		}
	})
}

// incoming is r carrying what Cloudflare knows of it (request.cf), as workers-go reads it. A request with none (the
// warm-up's, a WebSocket message the adapter made) is r as it is.
func incoming(r *http.Request) (out *http.Request) {
	// A WebSocket message: the adapter passes the upgrade's request.cf along, since its own request has none.
	if r.Header.Get(cfHeader) != "" {
		return withCF(r)
	}
	out = r
	defer func() { recover() }() // no request object behind this one: workers-go says so by panicking
	p, err := fetch.NewIncomingProperties(r.Context())
	if err != nil || p == nil {
		return r
	}
	cf := CF{
		Country: p.Country, Region: p.Region, RegionCode: p.RegionCode, City: p.City, PostalCode: p.PostalCode,
		Continent: p.Continent, Timezone: p.Timezone, Latitude: p.Latitude, Longitude: p.Longitude, Colo: p.Colo, ASN: p.Asn,
	}
	raw, _ := json.Marshal(cf)
	return r.WithContext(withValue(r.Context(), cfValue{cf, raw}))
}

// besideTheWasm is data the Worker's entry (build/worker.mjs, which charter build writes) put in a global before Go
// started: the docs' pages. "" when the app has none.
func besideTheWasm(name string) string {
	if v := js.Global().Get(name); v.Type() == js.TypeString {
		return v.String()
	}
	return ""
}
