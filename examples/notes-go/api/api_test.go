package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/go/auth"
	"github.com/joeblew999/charter/go/auth/authtest"
)

// A real HTTP server over the in-memory store and hub: the handlers as `go run .` serves them.
func server(t *testing.T) (*httptest.Server, *MemStore) {
	t.Helper()
	memory := &MemStore{}
	env := Env{
		Var:   testVars,
		Store: func() (Store, error) { return memory, nil },
		Hub:   func() (Hub, error) { return memory, nil },
	}
	srv := httptest.NewServer(Handler(env))
	t.Cleanup(srv.Close)
	return srv, memory
}

// testVars are the Worker's settings in the tests: its name and its two tokens.
func testVars(name string) string {
	return map[string]string{"APP_NAME": "test", "WRITE_TOKEN": "write-token", "READ_TOKEN": "read-token"}[name]
}

// do sends a request with the write token, unless headers set Authorization themselves.
func do(t *testing.T, method, url, body string, headers ...string) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer write-token")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var b strings.Builder
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		b.WriteString(scanner.Text() + "\n")
	}
	return res.StatusCode, b.String(), res.Header
}

func create(t *testing.T, base, body string) Note {
	t.Helper()
	status, out, _ := do(t, "POST", base+"/api/notes", `{"body":`+quote(body)+`}`)
	if status != 200 {
		t.Fatalf("create: HTTP %d %s", status, out)
	}
	var note Note
	if err := json.Unmarshal([]byte(out), &note); err != nil {
		t.Fatal(err)
	}
	return note
}

func quote(s string) string { b, _ := json.Marshal(s); return string(b) }

// Writing needs the write token: none (or an unknown one) is 401, the read token 403. Reads are public.
func TestTokens(t *testing.T) {
	srv, _ := server(t)
	for _, c := range []struct {
		method, path, body, authorization string
		want                              int
	}{
		{"POST", "/api/notes", `{"body":"x"}`, "", 401},
		{"POST", "/api/notes", `{"body":"x"}`, "Bearer wrong", 401},
		{"POST", "/api/notes", `{"body":"x"}`, "Bearer read-token", 403},
		{"POST", "/api/notes", `{"body":"x"}`, "Bearer write-token", 200},
		{"GET", "/api/notes", "", "", 200},
		{"GET", "/api/hello", "", "", 200},
	} {
		status, out, header := do(t, c.method, srv.URL+c.path, c.body, "Authorization", c.authorization)
		if status != c.want {
			t.Errorf("%s %s with %q: %d, want %d: %s", c.method, c.path, c.authorization, status, c.want, out)
		}
		if status == 401 && header.Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s %s: a 401 without WWW-Authenticate: Bearer", c.method, c.path)
		}
	}
}

// Cloudflare Access and an OIDC issuer (a test one, as both): a person writes, a machine's service
// token only reads, an OIDC token does what its scope says; one that does not verify is 401.
func TestAccessAndOIDC(t *testing.T) {
	issuer := authtest.New()
	defer issuer.Close()
	memory := &MemStore{}
	vars := map[string]string{"APP_NAME": "test", auth.AccessTeam: issuer.URL, auth.AccessAUD: "aud", auth.OIDCIssuer: issuer.URL, auth.OIDCAudience: "https://notes.test"}
	srv := httptest.NewServer(Handler(Env{
		Var:   func(name string) string { return vars[name] },
		Store: func() (Store, error) { return memory, nil },
		Hub:   func() (Hub, error) { return memory, nil },
	}))
	defer srv.Close()
	for _, c := range []struct {
		name, header, value string
		want                int
	}{
		{"a person", auth.AccessHeader, issuer.Access("aud", "dev@example.com", ""), 200},
		{"a machine", auth.AccessHeader, issuer.Access("aud", "", "ci.access"), 403},
		{"another application's", auth.AccessHeader, issuer.Access("other", "dev@example.com", ""), 401},
		{"an OIDC token with write", "Authorization", "Bearer " + issuer.OIDC("https://notes.test", "u1", "write"), 200},
		{"an OIDC token without", "Authorization", "Bearer " + issuer.OIDC("https://notes.test", "u1", "read"), 403},
		{"an OIDC token for another API", "Authorization", "Bearer " + issuer.OIDC("https://elsewhere.test", "u1", "write"), 401},
	} {
		status, out, _ := do(t, "POST", srv.URL+"/api/notes", `{"body":"x"}`, "Authorization", "", c.header, c.value)
		if status != c.want {
			t.Errorf("%s: %d, want %d: %s", c.name, status, c.want, out)
		}
	}
	// The spec's openIdConnectUrl sends a client on to the issuer.
	res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Get(srv.URL + "/.well-known/openid-configuration")
	if err != nil || res.StatusCode != 302 || res.Header.Get("Location") != issuer.URL+"/.well-known/openid-configuration" {
		t.Errorf("discovery: %v %v", res, err)
	}
}

func TestHello(t *testing.T) {
	srv, _ := server(t)
	status, body, _ := do(t, "GET", srv.URL+"/api/hello", "")
	if status != 200 || strings.TrimSpace(body) != `{"message":"Hello from test"}` {
		t.Fatalf("HTTP %d %s", status, body)
	}
}

func TestCreateAndListWithCursor(t *testing.T) {
	srv, _ := server(t)
	for _, body := range []string{"a", "b", "c"} {
		create(t, srv.URL, body)
	}
	_, page1, _ := do(t, "GET", srv.URL+"/api/notes?limit=2", "")
	if strings.TrimSpace(page1) == "" || !strings.Contains(page1, `"next_cursor":"2"`) || strings.Index(page1, `"id":3`) > strings.Index(page1, `"id":2`) {
		t.Fatalf("page 1 (newest first, next_cursor 2): %s", page1)
	}
	_, page2, _ := do(t, "GET", srv.URL+"/api/notes?limit=2&cursor=2", "")
	if !strings.Contains(page2, `"id":1`) || strings.Contains(page2, "next_cursor") {
		t.Fatalf("page 2 (the last: no next_cursor): %s", page2)
	}
}

func TestAnEmptyListIsAnArray(t *testing.T) {
	srv, _ := server(t)
	_, body, _ := do(t, "GET", srv.URL+"/api/notes", "")
	if strings.TrimSpace(body) != `{"data":[]}` {
		t.Fatalf("got %s", body)
	}
}

func TestInvalidInputIs422WithTheLocation(t *testing.T) {
	srv, _ := server(t)
	for _, c := range []struct{ method, path, body, location string }{
		{"GET", "/api/notes?limit=0", "", "query.limit"},
		{"GET", "/api/notes?limit=101", "", "query.limit"},
		{"GET", "/api/notes?cursor=x", "", "query.cursor"},
		{"GET", "/api/notes/watch?after=x", "", "query.after"},
		{"GET", "/api/notes/watch?seconds=301", "", "query.seconds"},
		{"POST", "/api/notes", `{"body":""}`, "body.body"},
		{"POST", "/api/notes", `{"body":"a ` + END + ` b"}`, "body.body"},
		{"POST", "/api/notes", `{}`, "body"},
	} {
		status, body, _ := do(t, c.method, srv.URL+c.path, c.body)
		if status != 422 || !strings.Contains(body, `"location":"`+c.location) {
			t.Errorf("%s %s %s: HTTP %d %s, want 422 at %s", c.method, c.path, c.body, status, body, c.location)
		}
	}
}

func TestUnknownPathAndMethod(t *testing.T) {
	srv, _ := server(t)
	if status, _, _ := do(t, "GET", srv.URL+"/api/nope", ""); status != 404 {
		t.Errorf("unknown path: HTTP %d, want 404", status)
	}
	status, _, header := do(t, "DELETE", srv.URL+"/api/notes", "")
	if status != 405 || header.Get("Allow") != "GET, POST" {
		t.Errorf("DELETE /api/notes: HTTP %d Allow %q, want 405 GET, POST", status, header.Get("Allow"))
	}
}

// The SSE wire format is the oRPC Worker's, byte for byte: a comment, then event/retry/id/data
// per note, then the close event carrying the terminator.
func TestWatchCatchesUpAndEndsWithTheTerminator(t *testing.T) {
	srv, _ := server(t)
	create(t, srv.URL, "one")
	two := create(t, srv.URL, "two")
	status, body, header := do(t, "GET", srv.URL+"/api/notes/watch?after=1&seconds=1", "")
	want := ": \n\nevent: message\nretry: 1000\nid: 2\ndata: {\"id\":2,\"body\":\"two\",\"created_at\":\"" + two.CreatedAt + "\"}\n\nevent: close\ndata: \"" + END + "\"\n\n"
	if status != 200 || header.Get("Content-Type") != "text/event-stream" || body != want {
		t.Fatalf("HTTP %d %s\n%q\nwant\n%q", status, header.Get("Content-Type"), body, want)
	}
}

func TestWatchWithoutAfterStartsFromNowAndGoesLive(t *testing.T) {
	srv, _ := server(t)
	create(t, srv.URL, "old")
	// Create the live note only once the stream has started, so a slow machine can't make the note
	// older than the stream (the check runs this beside a TinyGo build).
	res, err := http.Get(srv.URL + "/api/notes/watch?seconds=3")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	lines := bufio.NewScanner(res.Body)
	lines.Scan() // the opening comment: the response has started
	go func() { time.Sleep(300 * time.Millisecond); create(t, srv.URL, "new") }()
	var seen strings.Builder
	for lines.Scan() {
		seen.WriteString(lines.Text() + "\n")
	}
	body := seen.String()
	if strings.Contains(body, `"old"`) || !strings.Contains(body, "id: 2\n") {
		t.Fatalf("want only the live note 2:\n%s", body)
	}
}

func TestLastEventIDIsAPositionAndTheNewestWins(t *testing.T) {
	srv, _ := server(t)
	for _, body := range []string{"a", "b", "c"} {
		create(t, srv.URL, body)
	}
	_, body, _ := do(t, "GET", srv.URL+"/api/notes/watch?after=1&seconds=1", "", "Last-Event-ID", "2")
	if strings.Contains(body, "id: 2\n") || !strings.Contains(body, "id: 3\n") {
		t.Fatalf("want only note 3:\n%s", body)
	}
}

// failingHub never lets anyone subscribe.
type failingHub struct{}

func (failingHub) Publish(context.Context, Note) error { return nil }
func (failingHub) Subscribe(func(Note), func(error)) (func(), error) {
	return nil, errors.New("hub unavailable")
}

func TestWatchEndsWithoutTheTerminatorWhenTheHubIsDown(t *testing.T) {
	memory := &MemStore{}
	env := Env{Var: func(string) string { return "" }, Store: func() (Store, error) { return memory, nil }, Hub: func() (Hub, error) { return failingHub{}, nil }}
	srv := httptest.NewServer(Handler(env))
	defer srv.Close()
	status, body, _ := do(t, "GET", srv.URL+"/api/notes/watch?after=0&seconds=30", "")
	if status != 200 || strings.Contains(body, END) || strings.Contains(body, "event: error") {
		t.Fatalf("HTTP %d, want a stream that just ends (so SDKs reconnect):\n%s", status, body)
	}
}

func TestLiveNeedsAnUpgradeAndThenSendsOneNotePerLine(t *testing.T) {
	srv, _ := server(t)
	if status, _, _ := do(t, "GET", srv.URL+"/api/notes/live", ""); status != 426 {
		t.Fatalf("plain GET: HTTP %d, want 426", status)
	}
	if status, _, _ := do(t, "GET", srv.URL+"/api/notes/live?after=x", "", "Upgrade", "websocket"); status != 422 {
		t.Fatalf("bad after: HTTP %d, want 422", status)
	}
	create(t, srv.URL, "one")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/notes/live?after=0", nil)
	req.Header.Set("Upgrade", "websocket")
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	go func() { time.Sleep(200 * time.Millisecond); create(t, srv.URL, "two") }()
	var got []int64
	scanner := bufio.NewScanner(res.Body)
	for len(got) < 2 && scanner.Scan() {
		if scanner.Text() == "" {
			continue // padding: the response starts before the first note
		}
		var note Note
		if err := json.Unmarshal(scanner.Bytes(), &note); err != nil {
			t.Fatalf("line %q: %v", scanner.Text(), err)
		}
		got = append(got, note.ID)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("got %v, want [1 2]", got)
	}
}

// ---- the specs ----

func spec(t *testing.T, generate func(string) ([]byte, error)) map[string]any {
	t.Helper()
	raw, err := generate("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func at(doc any, path ...string) any {
	for _, key := range path {
		m, ok := doc.(map[string]any)
		if !ok {
			return nil
		}
		doc = m[key]
	}
	return doc
}

func TestTheWorkerServesBothSpecsWithItsOriginAsServer(t *testing.T) {
	srv, _ := server(t)
	_, openapi, _ := do(t, "GET", srv.URL+"/api/openapi.json", "")
	_, asyncapi, _ := do(t, "GET", srv.URL+"/api/asyncapi.json", "")
	host := strings.TrimPrefix(srv.URL, "http://")
	if !strings.Contains(openapi, `"servers":[{"url":"`+srv.URL+`"}]`) {
		t.Errorf("openapi servers: %s", openapi)
	}
	if !strings.Contains(asyncapi, `"host":"`+host+`","protocol":"ws"`) {
		t.Errorf("asyncapi servers: %s", asyncapi)
	}
}

func TestTheWebSocketChannelIsInAsyncAPINotOpenAPI(t *testing.T) {
	if at(spec(t, OpenAPI), "paths", "/api/notes/live") != nil {
		t.Error("/api/notes/live is in OpenAPI")
	}
	asyncapi := spec(t, AsyncAPI)
	if at(asyncapi, "channels", "liveNotes", "address") != "/api/notes/live" {
		t.Errorf("channel liveNotes: %v", at(asyncapi, "channels", "liveNotes"))
	}
	if at(asyncapi, "channels", "liveNotes", "bindings", "ws", "query", "properties", "after", "pattern") != `^\d+$` {
		t.Error("the channel's query binding has no `after` with its pattern")
	}
	if at(asyncapi, "operations", "receiveNote", "action") != "receive" {
		t.Error("no receive operation receiveNote")
	}
	if at(asyncapi, "components", "messages", "Note", "payload", "$ref") != "#/components/schemas/Note" || at(asyncapi, "components", "schemas", "Note", "properties", "id") == nil {
		t.Errorf("message Note: %v", at(asyncapi, "components"))
	}
}
