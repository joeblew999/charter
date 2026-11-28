// Package cloud is what Cloudflare gives a Go Worker that workers-go does not, or gives differently: for everything
// else, import workers-go's own packages (github.com/syumai/workers-go/cloudflare/…).
//
//	Database, KV, Bucket, Queue, Object,   what the Worker is bound to, as typed values: an app's are written
//	RateLimit, Mailer, Variable            from wrangler.jsonc into env/env.gen.go (names.go)
//	env.DB.Open(), env.WRITES.Allow(key)   a database for the queries; a Rate Limiting binding asked
//	env.MAIL.Send, ConsumeEmail            email out and in (email_js.go)
//	env.APP_NAME.Value()                   a variable of the Worker
//	HTTPClient()                           an http.Client over the Worker's fetch
//	Batch, and what queries.gen.go calls   D1 as typed rows and batches (d1.go)
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

// Binding is one name of the Worker's environment, and what kind of thing is bound there.
type Binding struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Set, for a variable or a secret: whether it has a value. The value is never given.
	Set bool `json:"set,omitempty"`
}

// The kinds of binding.
const (
	KindDatabase      = "database"
	KindKV            = "kv"
	KindBucket        = "bucket"
	KindQueue         = "queue"
	KindDurableObject = "durable object"
	KindRateLimit     = "rate limit"
	KindEmail         = "email"
	KindService       = "service"
	KindOther         = "other"
	KindText          = "text"
)

var kinds = []string{KindDatabase, KindKV, KindBucket, KindQueue, KindDurableObject, KindRateLimit, KindEmail, KindService, KindOther, KindText}

// ErrNoProbe is Probe's answer for a binding that has nothing harmless to ask.
var ErrNoProbe = errors.New("cloud: nothing to ask a binding of this kind")

// Probes reports whether Probe has something to ask a binding of this kind.
func Probes(kind string) bool { return kind == KindDatabase || kind == KindKV || kind == KindBucket }
