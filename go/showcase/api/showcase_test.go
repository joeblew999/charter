package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/joeblew999/charter/go/transport"
)

// A real HTTP server with the WebSocket adapter around the handlers: what `go run ./cmd/showcase` serves.
func server(t *testing.T, settings map[string]string) *httptest.Server {
	t.Helper()
	env := Env{Var: func(name string) string { return settings[name] }, HTTP: http.DefaultClient}
	srv := httptest.NewServer(transport.Serve(Handler(env)))
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, method, url, contentType, body string, headers ...string) (int, string, http.Header) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(out), res.Header
}

const (
	form = "application/x-www-form-urlencoded"
	js   = "application/json"
)

// token gets an access token the way the SDKs do.
func token(t *testing.T, base string) string {
	t.Helper()
	status, body, _ := do(t, "POST", base+"/oauth/token", form, "client_id=id-1&client_secret=secret-1")
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil || status != 200 || out.AccessToken == "" || out.ExpiresIn != 3600 {
		t.Fatalf("token: HTTP %d %s", status, body)
	}
	return out.AccessToken
}

func TestTheTokenEndpointTakesAFormAndRefusesAnotherClient(t *testing.T) {
	srv := server(t, nil)
	token(t, srv.URL)
	for body, want := range map[string]int{
		"client_id=id-1&client_secret=nope":     401,
		"client_id=nope&client_secret=secret-1": 401,
		"client_id=id-1":                        422,
	} {
		if status, out, _ := do(t, "POST", srv.URL+"/oauth/token", form, body); status != want {
			t.Errorf("%s: HTTP %d %s, want %d", body, status, out, want)
		}
	}
	// The client is a setting: on Cloudflare a secret.
	other := server(t, map[string]string{"CLIENT_ID": "other", "CLIENT_SECRET": "s3"})
	if status, _, _ := do(t, "POST", other.URL+"/oauth/token", form, "client_id=id-1&client_secret=secret-1"); status != 401 {
		t.Errorf("the default client on a server with its own: HTTP %d, want 401", status)
	}
	if status, _, _ := do(t, "POST", other.URL+"/oauth/token", form, "client_id=other&client_secret=s3"); status != 200 {
		t.Errorf("the server's own client: HTTP %d, want 200", status)
	}
}

func TestEveryOperationButTheTokenOneNeedsAValidToken(t *testing.T) {
	srv := server(t, nil)
	good := token(t, srv.URL)
	env := Env{}
	expired := env.newToken(time.Now().Add(-2 * tokenLife))
	foreign := Env{Var: func(string) string { return "another secret" }}.newToken(time.Now())
	for _, c := range []struct{ method, path, contentType, body string }{
		{"GET", "/notes", "", ""},
		{"POST", "/notes", js, `{"body":"x"}`},
		{"POST", "/files", "multipart/form-data; boundary=x", "--x--\r\n"},
		{"POST", "/chat", js, `{"prompt":"x"}`},
		{"GET", livePath, "", ""},
		{"POST", livePath, js, `{"topic":"x"}`},
	} {
		for name, authorization := range map[string]string{"no token": "", "a made-up token": "Bearer nope", "an expired token": "Bearer " + expired, "another server's token": "Bearer " + foreign, "not a bearer": "Basic " + good} {
			status, body, header := do(t, c.method, srv.URL+c.path, c.contentType, c.body, "Authorization", authorization)
			if status != 401 || header.Get("WWW-Authenticate") != "Bearer" {
				t.Errorf("%s %s with %s: HTTP %d %s, want 401", c.method, c.path, name, status, body)
			}
		}
		if status, body, _ := do(t, c.method, srv.URL+c.path, c.contentType, c.body, "Authorization", "Bearer "+good); status == 401 {
			t.Errorf("%s %s with a valid token: HTTP 401 %s", c.method, c.path, body)
		}
	}
	// ?access_token= is for the WebSocket only.
	if status, _, _ := do(t, "GET", srv.URL+"/notes?access_token="+good, "", ""); status != 401 {
		t.Errorf("GET /notes with ?access_token=: HTTP %d, want 401", status)
	}
}

func TestListPagesWithACursor(t *testing.T) {
	srv := server(t, nil)
	auth := []string{"Authorization", "Bearer " + token(t, srv.URL)}
	for path, want := range map[string]string{
		"/notes":                  `{"data":[{"id":"n1","body":"note n1"},{"id":"n2","body":"note n2"}],"next_cursor":"2"}`,
		"/notes?cursor=2":         `{"data":[{"id":"n3","body":"note n3"},{"id":"n4","body":"note n4"}],"next_cursor":"4"}`,
		"/notes?cursor=4":         `{"data":[{"id":"n5","body":"note n5"}]}`,
		"/notes?cursor=5":         `{"data":[]}`,
		"/notes?limit=4&cursor=1": `{"data":[{"id":"n2","body":"note n2"},{"id":"n3","body":"note n3"},{"id":"n4","body":"note n4"},{"id":"n5","body":"note n5"}]}`,
	} {
		if status, body, _ := do(t, "GET", srv.URL+path, "", "", auth...); status != 200 || strings.TrimSpace(body) != want {
			t.Errorf("%s: HTTP %d %s\nwant %s", path, status, body, want)
		}
	}
	for path, location := range map[string]string{"/notes?cursor=x": "query.cursor", "/notes?cursor=6": "query.cursor", "/notes?limit=0": "query.limit", "/notes?limit=101": "query.limit"} {
		if status, body, _ := do(t, "GET", srv.URL+path, "", "", auth...); status != 422 || !strings.Contains(body, `"location":"`+location+`"`) {
			t.Errorf("%s: HTTP %d %s, want 422 at %s", path, status, body, location)
		}
	}
}

func TestCreateIsIdempotentByItsKey(t *testing.T) {
	srv := server(t, nil)
	auth := []string{"Authorization", "Bearer " + token(t, srv.URL)}
	_, first, _ := do(t, "POST", srv.URL+"/notes", js, `{"body":"hello"}`, append(auth, "Idempotency-Key", "key-1")...)
	_, again, _ := do(t, "POST", srv.URL+"/notes", js, `{"body":"hello"}`, append(auth, "Idempotency-Key", "key-1")...)
	if strings.TrimSpace(first) != `{"id":"key-1","body":"hello"}` || again != first {
		t.Errorf("first %s, again %s", first, again)
	}
	if _, without, _ := do(t, "POST", srv.URL+"/notes", js, `{"body":"hello"}`, auth...); strings.TrimSpace(without) != `{"id":"new","body":"hello"}` {
		t.Errorf("without a key: %s", without)
	}
	if status, body, _ := do(t, "POST", srv.URL+"/notes", js, `{}`, auth...); status != 422 {
		t.Errorf("without a body field: HTTP %d %s, want 422", status, body)
	}
}

// The receiver checks the signature the way the SDKs' helper does: HMAC-SHA256 of the exact body,
// in hex, under the shared secret.
func TestCreateSendsTheSignedWebhook(t *testing.T) {
	type delivery struct{ body, signature, contentType string }
	deliveries := make(chan delivery, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		deliveries <- delivery{string(body), r.Header.Get(SignatureHeader), r.Header.Get("Content-Type")}
	}))
	defer receiver.Close()
	srv := server(t, map[string]string{"WEBHOOK_URL": receiver.URL, "WEBHOOK_SECRET": "shared"})
	do(t, "POST", srv.URL+"/notes", js, `{"body":"hello"}`, "Authorization", "Bearer "+token(t, srv.URL), "Idempotency-Key", "key-1")
	select {
	case got := <-deliveries:
		mac := hmac.New(sha256.New, []byte("shared"))
		mac.Write([]byte(got.body))
		if got.signature != hex.EncodeToString(mac.Sum(nil)) || got.contentType != js || got.body != `{"event":"note.created","note":{"id":"key-1","body":"hello"}}` {
			t.Errorf("delivery %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no webhook within 5 s")
	}
}

func TestCreateWorksWhenTheWebhookReceiverIsDown(t *testing.T) {
	srv := server(t, map[string]string{"WEBHOOK_URL": "http://127.0.0.1:1/nobody"})
	if status, body, _ := do(t, "POST", srv.URL+"/notes", js, `{"body":"hello"}`, "Authorization", "Bearer "+token(t, srv.URL)); status != 200 {
		t.Errorf("HTTP %d %s, want 200", status, body)
	}
}

func upload(t *testing.T, base, auth string, write func(*multipart.Writer)) (int, string) {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	write(form)
	form.Close()
	status, out, _ := do(t, "POST", base+"/files", form.FormDataContentType(), body.String(), "Authorization", auth)
	return status, out
}

func TestUploadTakesAMultipartForm(t *testing.T) {
	srv := server(t, nil)
	auth := "Bearer " + token(t, srv.URL)
	big := strings.Repeat("x", 100_000) // more than Huma keeps in memory by default
	status, body := upload(t, srv.URL, auth, func(form *multipart.Writer) {
		file, _ := form.CreateFormFile("file", "hello.txt")
		io.WriteString(file, big)
		form.WriteField("note", "a note")
	})
	if status != 200 || strings.TrimSpace(body) != `{"id":"hello.txt:a note","size":100000}` {
		t.Errorf("HTTP %d %s", status, body)
	}
	status, body = upload(t, srv.URL, auth, func(form *multipart.Writer) {
		file, _ := form.CreateFormFile("file", "hello.txt")
		io.WriteString(file, "hi")
	})
	if status != 200 || strings.TrimSpace(body) != `{"id":"hello.txt:","size":2}` {
		t.Errorf("without a note: HTTP %d %s", status, body)
	}
	status, body = upload(t, srv.URL, auth, func(form *multipart.Writer) { form.WriteField("note", "a note") })
	if status != 422 || !strings.Contains(body, `"location":"body.file"`) {
		t.Errorf("without a file: HTTP %d %s, want 422 at body.file", status, body)
	}
}

func TestChatStreamsChunksAsSSE(t *testing.T) {
	srv := server(t, nil)
	status, body, header := do(t, "POST", srv.URL+"/chat", js, `{"prompt":"from go"}`, "Authorization", "Bearer "+token(t, srv.URL))
	want := "data: {\"text\":\"echo\",\"done\":false}\n\ndata: {\"text\":\"from\",\"done\":false}\n\ndata: {\"text\":\"go\",\"done\":true}\n\n"
	if status != 200 || header.Get("Content-Type") != "text/event-stream" || body != want {
		t.Errorf("HTTP %d %s\n%q\nwant\n%q", status, header.Get("Content-Type"), body, want)
	}
}

// The channel as the adapters see it: the upgrade is answered 204 with the header that says it takes
// messages, and a message is a POST answered with one event per line.
func TestTheChannelAnswersTheUpgradeAndEachMessage(t *testing.T) {
	srv := server(t, nil)
	good := token(t, srv.URL)
	if status, _, _ := do(t, "GET", srv.URL+livePath, "", "", "Authorization", "Bearer "+good); status != 426 {
		t.Errorf("a plain GET: HTTP %d, want 426", status)
	}
	// Without the adapter: what Go itself answers the upgrade with.
	req := httptest.NewRequest("GET", livePath+"?access_token="+good, nil)
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	Handler(Env{}).ServeHTTP(rec, req)
	if rec.Code != 204 || rec.Header().Get(transport.MessagesHeader) != transport.MessagesPost {
		t.Errorf("the upgrade: HTTP %d, %s %q", rec.Code, transport.MessagesHeader, rec.Header().Get(transport.MessagesHeader))
	}
	status, body, _ := do(t, "POST", srv.URL+livePath+"?access_token="+good, js, `{"topic":"notes"}`)
	want := `{"event":"notes.created","id":"n1","body":"note n1","auth":"Bearer ` + good + `"}` + "\n" +
		`{"event":"notes.created","id":"n2","body":"note n2","auth":"Bearer ` + good + `"}` + "\n" +
		`{"event":"notes.created","id":"n3","body":"note n3","auth":"Bearer ` + good + `"}` + "\n"
	if status != 200 || body != want {
		t.Errorf("a message: HTTP %d\n%s\nwant\n%s", status, body, want)
	}
	if status, body, _ := do(t, "POST", srv.URL+livePath, js, `{"subject":"x"}`, "Authorization", "Bearer "+good); status != 422 {
		t.Errorf("a message that is not a subscribe: HTTP %d %s, want 422", status, body)
	}
}

// The whole way, as a client: a WebSocket, a subscribe message, the topic's events.
func TestTheWebSocketBothWays(t *testing.T) {
	srv := server(t, nil)
	good := token(t, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	address := strings.Replace(srv.URL, "http", "ws", 1) + livePath
	if _, res, err := websocket.Dial(ctx, address, nil); err == nil || res == nil || res.StatusCode != 401 {
		t.Fatalf("without a token: %v %+v, want 401", err, res)
	}
	for name, dial := range map[string]func() (*websocket.Conn, *http.Response, error){
		"Authorization": func() (*websocket.Conn, *http.Response, error) {
			return websocket.Dial(ctx, address, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + good}}})
		},
		"access_token": func() (*websocket.Conn, *http.Response, error) {
			return websocket.Dial(ctx, address+"?access_token="+url.QueryEscape(good), nil)
		},
	} {
		conn, _, err := dial()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defer conn.CloseNow()
		for _, topic := range []string{"notes", "other"} {
			if err := conn.Write(ctx, websocket.MessageText, []byte(`{"topic":"`+topic+`"}`)); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"n1", "n2", "n3"} {
				_, data, err := conn.Read(ctx)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				var event NoteEvent
				if err := json.Unmarshal(data, &event); err != nil || event.Event != topic+".created" || event.ID != id || event.Auth != "Bearer "+good {
					t.Errorf("%s: %s, want %s.created %s", name, data, topic, id)
				}
			}
		}
		// What the contract refuses closes the socket.
		conn.Write(ctx, websocket.MessageText, []byte(`{"subject":"x"}`))
		if _, _, err := conn.Read(ctx); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
			t.Errorf("%s: a refused message: %v, want close 1008", name, err)
		}
	}
}

func TestUnknownPathAndMethod(t *testing.T) {
	srv := server(t, nil)
	if status, _, _ := do(t, "GET", srv.URL+"/nope", "", ""); status != 404 {
		t.Errorf("unknown path: HTTP %d, want 404", status)
	}
	if status, _, header := do(t, "DELETE", srv.URL+"/notes", "", ""); status != 405 || header.Get("Allow") != "GET, POST" {
		t.Errorf("DELETE /notes: HTTP %d Allow %q, want 405 GET, POST", status, header.Get("Allow"))
	}
}

func TestTheServerGivesBothSpecsWithItsOriginAsServer(t *testing.T) {
	srv := server(t, nil)
	_, openapi, _ := do(t, "GET", srv.URL+"/openapi.json", "", "")
	_, asyncapi, _ := do(t, "GET", srv.URL+"/asyncapi.json", "", "")
	if !strings.Contains(openapi, `"servers":[{"url":"`+srv.URL+`"}]`) {
		t.Errorf("openapi servers: %s", openapi)
	}
	if !strings.Contains(asyncapi, `"host":"`+strings.TrimPrefix(srv.URL, "http://")+`","protocol":"ws"`) {
		t.Errorf("asyncapi servers: %s", asyncapi)
	}
}
