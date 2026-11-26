// Package cloud is what Cloudflare gives a Go Worker that workers-go does not, or gives differently: for everything
// else, import workers-go's own packages (github.com/syumai/workers-go/cloudflare/…).
//
//	Var("APP_ENV")                         a variable of the Worker
//	Allow("WRITES", key)                   a Rate Limiting binding
//	HTTPClient()                           an http.Client over the Worker's fetch
//	SendEmail, ConsumeEmail                email out and in (email_js.go)
//	Open("DB"), Query, One, Exec, Batch    D1 as typed rows and batches (d1.go)
//
// A Worker runs on Cloudflare: on wrangler dev locally, deployed remotely. Nothing here runs anywhere else. The
// package still compiles for the machine it is written on (other.go), so editors, go vet and the program that
// writes an API's specs need no special setup; every call there answers ErrNotCloudflare.
package cloud

import (
	"errors"
	"net/http"
)

// ErrNoBinding means the Worker has no binding of that name (wrangler.jsonc). Allow still allows (it fails open, so
// a missing binding never takes an app down): log it.
var ErrNoBinding = errors.New("cloud: binding not configured")

// ErrNotCloudflare is what every call answers in a program that is not a Worker.
var ErrNotCloudflare = errors.New("cloud: not running on Cloudflare (go tool charter dev, or deployed)")

// ClientKey is a request's caller for a per-client limit: the client's IP, which Cloudflare puts in CF-Connecting-IP,
// or "local" without one.
func ClientKey(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	return "local"
}
