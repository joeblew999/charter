package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/joeblew999/charter/go/auth"
)

type out struct{ Body string }

var (
	writes = Limit{Binding: "WRITE_LIMIT", Limit: 2, Period: 60, Scope: "write"}
	hellos = Limit{Binding: "HELLO_LIMIT", Limit: 3, Period: 10}
)

// An API with a public hello limited per IP (its own limit), a list with no limit, and a create
// that needs write, limited per caller by the write scope's limit; memory counts, on a clock the
// test moves.
func testAPI(t *testing.T, now *time.Time) humatest.TestAPI {
	_, api := humatest.New(t)
	auth.Scheme(api.OpenAPI())
	Declare(api.OpenAPI(), writes)
	api.UseMiddleware(auth.Middleware(api, settings{"WRITE_TOKEN": "w", "OTHER_TOKEN": "o"}.Get,
		auth.Token{Secret: "WRITE_TOKEN", Scopes: []string{"write"}},
		auth.Token{Secret: "OTHER_TOKEN", Scopes: []string{"write"}}))
	memory := &Memory{now: func() time.Time { return *now }}
	api.UseMiddleware(Middleware(api, memory.Allow))
	handler := func(context.Context, *struct{}) (*out, error) { return &out{Body: "ok"}, nil }
	huma.Register(api, huma.Operation{OperationID: "hello", Method: http.MethodGet, Path: "/hello", Extensions: On(hellos, nil)}, handler)
	huma.Register(api, huma.Operation{OperationID: "list", Method: http.MethodGet, Path: "/notes"}, handler)
	huma.Register(api, huma.Operation{OperationID: "create", Method: http.MethodPost, Path: "/notes", Security: auth.Needs("write")}, handler)
	return api
}

type settings map[string]string

func (s settings) Get(name string) string { return s[name] }

func TestLimitsPerCallerAndPerIP(t *testing.T) {
	now := time.Unix(1_000_000_020, 0)
	api := testAPI(t, &now)
	call := func(method, path string, headers ...any) (int, string) {
		resp := api.Do(method, path, headers...)
		return resp.Code, resp.Header().Get("Retry-After")
	}
	// The write scope's limit, per token: the third call is refused; another caller is not.
	for i, want := range []int{200, 200, 429} {
		if code, _ := call("POST", "/notes", "Authorization: Bearer w"); code != want {
			t.Errorf("write %d: %d, want %d", i+1, code, want)
		}
	}
	if code, retry := call("POST", "/notes", "Authorization: Bearer w"); code != 429 || retry != "60" {
		t.Errorf("over the limit: %d, Retry-After %q; want 429, 60", code, retry)
	}
	if code, _ := call("POST", "/notes", "Authorization: Bearer o"); code != 200 {
		t.Errorf("another token: %d, want 200", code)
	}
	// Refused by auth: not counted, and still 401.
	if code, _ := call("POST", "/notes"); code != 401 {
		t.Errorf("no credentials: %d, want 401", code)
	}
	// The public hello, per IP.
	for i := range 3 {
		if code, _ := call("GET", "/hello", "CF-Connecting-IP: 203.0.113.7"); code != 200 {
			t.Errorf("hello %d: %d", i+1, code)
		}
	}
	if code, retry := call("GET", "/hello", "CF-Connecting-IP: 203.0.113.7"); code != 429 || retry != "10" {
		t.Errorf("hello over the limit: %d, Retry-After %q", code, retry)
	}
	if code, _ := call("GET", "/hello", "CF-Connecting-IP: 203.0.113.8"); code != 200 {
		t.Errorf("hello from another IP: %d", code)
	}
	// No limit: never refused.
	for range 10 {
		if code, _ := call("GET", "/notes"); code != 200 {
			t.Fatalf("list: %d", code)
		}
	}
	// A new window.
	now = now.Add(60 * time.Second)
	if code, _ := call("POST", "/notes", "Authorization: Bearer w"); code != 200 {
		t.Errorf("write in the next window: %d, want 200", code)
	}
	if code, _ := call("GET", "/hello", "CF-Connecting-IP: 203.0.113.7"); code != 200 {
		t.Errorf("hello in the next window: %d, want 200", code)
	}
}

// The specs say each limited operation's limit and its 429 with Retry-After; the others say nothing.
func TestSpec(t *testing.T) {
	now := time.Now()
	api := testAPI(t, &now)
	raw, err := json.Marshal(api.OpenAPI())
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Limit     *Limit `json:"x-rate-limit"`
			Responses map[string]struct {
				Headers map[string]any            `json:"headers"`
				Content map[string]map[string]any `json:"content"`
			} `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		path, method string
		want         *Limit
	}{{"/hello", "get", &hellos}, {"/notes", "post", &writes}, {"/notes", "get", nil}} {
		op := doc.Paths[c.path][c.method]
		_, has429 := op.Responses["429"]
		if c.want == nil {
			if op.Limit != nil || has429 {
				t.Errorf("%s %s: a limit or a 429 it does not have", c.method, c.path)
			}
			continue
		}
		if op.Limit == nil || *op.Limit != *c.want {
			t.Errorf("%s %s: x-rate-limit %+v, want %+v", c.method, c.path, op.Limit, *c.want)
		}
		r := op.Responses["429"]
		if r.Headers["Retry-After"] == nil || !strings.Contains(string(raw), `"application/problem+json"`) || len(r.Content) == 0 {
			t.Errorf("%s %s: 429 %+v", c.method, c.path, r)
		}
	}
}

// Without a caller, the key is the client's IP: Cloudflare's header, else the connection's.
func TestKeys(t *testing.T) {
	for _, c := range []struct {
		caller       *auth.Caller
		header, addr string
		want         string
	}{
		{&auth.Caller{Scheme: "access", Machine: "ci.access", Subject: "s"}, "", "", "machine:ci.access"},
		{&auth.Caller{Scheme: "oidc", Subject: "user-1", Email: "a@b.c"}, "", "", "oidc:user-1"},
		{&auth.Caller{Scheme: "access", Email: "a@b.c"}, "", "", "email:a@b.c"},
		{&auth.Caller{Scheme: auth.Name, Token: "WRITE_TOKEN"}, "", "", "token:WRITE_TOKEN"},
		{nil, "203.0.113.7", "192.0.2.1:1234", "ip:203.0.113.7"},
		{nil, "", "192.0.2.1:1234", "ip:192.0.2.1"},
		{nil, "", "", "ip:unknown"},
	} {
		if got := keyOf(c.caller, c.header, c.addr); got != c.want {
			t.Errorf("%+v %q %q: key %q, want %q", c.caller, c.header, c.addr, got, c.want)
		}
	}
}
