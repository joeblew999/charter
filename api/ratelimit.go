// Package ratelimit is how often a caller may call a Huma API's operations, beside auth: the
// contract declares each limit, and one middleware enforces it, so the specs, the SDKs and the
// server agree.
//
// A limit is a Cloudflare Workers Rate Limiting binding (the Worker's config names it, with the
// same limit and period: charter.toml binds them from the spec), counted per caller: the
// key is who auth let in ([CallerOf]: the service token, the subject, the email or the
// token's secret), or the client's IP (CF-Connecting-IP) for an operation that is public. An
// operation has a limit of its own ([On], in its Extensions), or the limit of a scope it needs
// ([DeclareLimits]). Over it: 429, with Retry-After the binding's period, and the specs say so.
//
// The binding is a fixed window per key, local to each Cloudflare location and eventually
// consistent: good for stopping abuse, not for accounting
// (https://developers.cloudflare.com/workers/runtime-apis/bindings/rate-limit/). A Worker without
// the binding lets every call through and logs it. Built for the host (go run, go test), the
// counting is in memory, the same fixed window.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/cloud"
)

// extension is where an operation's limit is in the specs.
const extension = "x-rate-limit"

// Limit is a limit on calls: the Worker's Rate Limiting binding that counts, which says how many calls a caller
// may make in how long (the app's env.WRITE_LIMIT, written from wrangler.jsonc, where the limit is said once).
type Limit struct {
	Binding cloud.RateLimit
	Scope   Scope // Declare: every operation that needs this scope
}

// MarshalJSON is the limit as the specs have it (x-rate-limit): the binding's name, the calls and the seconds.
func (l Limit) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Binding string `json:"binding"`
		Limit   int    `json:"limit"`
		Period  int    `json:"period"`
		Scope   string `json:"scope,omitempty"`
	}{l.Binding.Name(), l.Binding.Calls(), l.Binding.Seconds(), l.Scope.String()})
}

// UnmarshalJSON reads a limit as the specs have it.
func (l *Limit) UnmarshalJSON(b []byte) error {
	var in struct {
		Binding string `json:"binding"`
		Limit   int    `json:"limit"`
		Period  int    `json:"period"`
		Scope   string `json:"scope"`
	}
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	*l = Limit{Binding: cloud.BoundRateLimit(in.Binding, in.Limit, in.Period), Scope: app.NewScope(in.Scope)}
	return nil
}

// Limited is an operation's own limit, for its Extensions (with Fern's, if it has them):
// charter-lib/ratelimit's limited, in TypeScript.
func Limited(limit Limit, extensions map[string]any) map[string]any {
	out := map[string]any{extension: limit}
	for key, value := range extensions {
		out[key] = value
	}
	return out
}

// DeclareLimits has the specs say each limited operation's limit and its 429 (config.OpenAPI): those
// with their own (On), and every other operation that needs the scope of one of perScope.
func DeclareLimits(spec *huma.OpenAPI, perScope ...Limit) {
	spec.OnAddOperation = append(spec.OnAddOperation, func(spec *huma.OpenAPI, op *huma.Operation) {
		limit, ok := limitOf(op)
		if !ok {
			security := op.Security
			if security == nil {
				security = spec.Security
			}
			for _, l := range perScope {
				if needs(security, l.Scope.String()) {
					limit, ok = l, true
					break
				}
			}
		}
		if !ok {
			return
		}
		if op.Extensions == nil {
			op.Extensions = map[string]any{}
		}
		op.Extensions[extension] = limit
		if op.Responses == nil {
			op.Responses = map[string]*huma.Response{}
		}
		op.Responses[strconv.Itoa(http.StatusTooManyRequests)] = tooMany(spec, limit)
	})
}

// needs is whether every alternative of security needs scope.
func needs(security []map[string][]string, scope string) bool {
	if scope == "" || len(security) == 0 {
		return false
	}
	for _, alternative := range security {
		found := false
		for _, scopes := range alternative {
			found = found || slices.Contains(scopes, scope)
		}
		if !found {
			return false
		}
	}
	return true
}

// tooMany is the 429 in the specs: Huma's error body, and Retry-After.
func tooMany(spec *huma.OpenAPI, limit Limit) *huma.Response {
	example := huma.NewError(0, "")
	contentType := "application/json"
	if f, ok := example.(huma.ContentTypeFilter); ok {
		contentType = f.ContentType(contentType)
	}
	t := reflect.TypeOf(example)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return &huma.Response{
		Description: fmt.Sprintf("Too Many Requests: more than %d calls in %d s by this caller", limit.Binding.Calls(), limit.Binding.Seconds()),
		Headers: map[string]*huma.Param{"Retry-After": {
			Description: "Seconds to wait before calling again",
			Schema:      &huma.Schema{Type: huma.TypeInteger},
		}},
		Content: map[string]*huma.MediaType{contentType: {Schema: spec.Components.Schemas.Schema(t, true, "Error")}},
	}
}

// limitOf is the limit an operation's Extensions hold.
func limitOf(op *huma.Operation) (Limit, bool) {
	limit, ok := op.Extensions[extension].(Limit)
	return limit, ok && limit.Binding.Name() != ""
}

// Limiter says whether key may make one more call under limit. allow (the Worker's binding, or
// memory on the host) is the one LimitMiddleware uses unless given another.
type Limiter func(ctx context.Context, limit Limit, key string) (bool, error)

// LimitMiddleware enforces each operation's limit, after auth's middleware (it keys on the caller):
// routes.UseMiddleware(AuthMiddleware(...)); routes.UseMiddleware(LimitMiddleware(routes, nil)).
// A nil limiter is allow.
func LimitMiddleware(api huma.API, limiter Limiter) func(huma.Context, func(huma.Context)) {
	if limiter == nil {
		limiter = allow
	}
	var warned sync.Map
	return func(ctx huma.Context, next func(huma.Context)) {
		limit, ok := limitOf(ctx.Operation())
		if !ok {
			next(ctx)
			return
		}
		allowed, err := limiter(ctx.Context(), limit, countedAs(ctx))
		if err != nil {
			// Fail open: a missing or failing binding never takes the API down.
			if _, seen := warned.LoadOrStore(limit.Binding.Name()+err.Error(), true); !seen {
				log.Printf("ratelimit: %s: %v; calls are not limited", limit.Binding.Name(), err)
			}
			allowed = true
		}
		if !allowed {
			ctx.SetHeader("Retry-After", strconv.Itoa(limit.Binding.Seconds()))
			huma.WriteErr(api, ctx, http.StatusTooManyRequests, fmt.Sprintf("too many calls: at most %d in %d s; retry after %d s", limit.Binding.Calls(), limit.Binding.Seconds(), limit.Binding.Seconds()))
			return
		}
		next(ctx)
	}
}

// countedAs is whom a call counts against: the caller auth let in, by the most specific thing that names
// it, or else the client's IP (CF-Connecting-IP on Cloudflare, the connection's address on the host).
func countedAs(ctx huma.Context) string {
	var caller *Caller
	if c, ok := CallerOf(ctx.Context()); ok {
		caller = &c
	}
	return keyOf(caller, ctx.Header("CF-Connecting-IP"), ctx.RemoteAddr())
}

func keyOf(c *Caller, ip, remoteAddr string) string {
	if c != nil {
		switch {
		case c.Machine != "":
			return "machine:" + c.Machine
		case c.Subject != "":
			return c.Scheme + ":" + c.Subject
		case c.Email != "":
			return "email:" + c.Email
		case c.Token != "":
			return "token:" + c.Token
		}
	}
	if ip != "" {
		return "ip:" + ip
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil && host != "" {
		return "ip:" + host
	}
	return "ip:unknown"
}

// MemoryLimiter counts in this process: a fixed window of Period per binding and key, as the binding does.
type MemoryLimiter struct {
	mu      sync.Mutex
	windows map[string]window
	now     func() time.Time
}

type window struct {
	end   time.Time
	count int
}

// Allow is a Limiter.
func (m *MemoryLimiter) Allow(_ context.Context, limit Limit, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if m.now != nil {
		now = m.now()
	}
	if m.windows == nil {
		m.windows = map[string]window{}
	}
	id := limit.Binding.Name() + "\x00" + key
	w := m.windows[id]
	if !now.Before(w.end) {
		// Drop the windows that have ended, so the map holds only current callers.
		for k, old := range m.windows {
			if !now.Before(old.end) {
				delete(m.windows, k)
			}
		}
		period := time.Duration(limit.Binding.Seconds()) * time.Second
		w = window{end: now.Truncate(period).Add(period)}
	}
	w.count++
	m.windows[id] = w
	return w.count <= limit.Binding.Calls(), nil
}
