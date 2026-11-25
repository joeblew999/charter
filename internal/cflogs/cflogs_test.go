package cflogs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRequest: a query as the API takes it: the Worker, the time asked for, and each filter that is set.
func TestRequest(t *testing.T) {
	now := time.UnixMilli(1795582823000)
	got, err := request(Query{Script: "app", Since: time.Hour, Level: "error", Search: "boom", Status: 500, Limit: 7}, now)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(got)
	want := `{"dry":true,"limit":7,"parameters":{"datasets":["cloudflare-workers"],"filterCombination":"and","filters":[` +
		`{"key":"$metadata.service","operation":"eq","type":"string","value":"app"},` +
		`{"key":"$metadata.level","operation":"eq","type":"string","value":"error"},` +
		`{"key":"$workers.event.response.status","operation":"eq","type":"number","value":500}],` +
		`"needle":{"matchCase":false,"value":"boom"}},"queryId":"charter-logs","timeframe":{"from":1795579223000,"to":1795582823000},"view":"events"}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
	plain, _ := request(Query{Script: "app"}, now)
	if plain["limit"] != 50 || plain["timeframe"].(map[string]int64)["from"] != now.Add(-10*time.Minute).UnixMilli() {
		t.Errorf("the defaults: %v", plain)
	}
	if _, err := request(Query{}, now); err == nil {
		t.Error("a query that names no Worker was made")
	}
	if _, err := request(Query{Script: "app", Limit: MaxLimit + 1}, now); err == nil {
		t.Error("a limit over the API's was taken")
	}
}

// TestEvents: the API's events, newest first, come back oldest first: a request's invocation log, and a console
// message whose source is a string.
func TestEvents(t *testing.T) {
	var asked map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/acc/workers/observability/telemetry/query" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, `{"success":false,"errors":[{"code":10000,"message":"auth"}]}`, 403)
			return
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &asked)
		io.WriteString(w, `{"success":true,"result":{"events":{"count":2,"events":[
			{"timestamp":1795582823047,"source":"publish failed","$metadata":{"level":"error","requestId":"r2"},"$workers":{"scriptName":"app","outcome":"exception","event":{"request":{"method":"POST","url":"https://app.example/add"},"response":{"status":500}}}},
			{"timestamp":1795582170226,"source":{"level":"info","message":"GET https://app.example/"},"$metadata":{"requestId":"r1"},"$workers":{"scriptName":"app","outcome":"ok","event":{"request":{"method":"GET","url":"https://app.example/"},"response":{"status":200}}}}]}}}`)
	}))
	defer srv.Close()
	c := &Client{Token: "tok", AccountID: "acc", BaseURL: srv.URL}
	events, err := c.Events(context.Background(), Query{Script: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if asked["view"] != "events" {
		t.Errorf("asked for the view %v", asked["view"])
	}
	if len(events) != 2 {
		t.Fatalf("%d events", len(events))
	}
	if got, want := Format(events[0]), "04:49:30.226 info  200 GET https://app.example/"; got != want {
		t.Errorf("the request:\n got %q\nwant %q", got, want)
	}
	if got, want := Format(events[1]), "05:00:23.047 error 500 (exception) POST https://app.example/add: publish failed"; got != want {
		t.Errorf("the error:\n got %q\nwant %q", got, want)
	}
	if _, err := (&Client{Token: "no", AccountID: "acc", BaseURL: srv.URL}).Events(context.Background(), Query{Script: "app"}); err == nil {
		t.Error("a refused token gave no error")
	}
}
