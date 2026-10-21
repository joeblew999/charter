package cftail_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/joeblew999/charter/go/lib/internal/cftail"
)

const sample = `{"outcome":"ok","scriptName":"app","eventTimestamp":1789300000123,
  "exceptions":[{"name":"Error","message":"boom","timestamp":1789300000124}],
  "logs":[{"message":["publish lobby v3:",{"code":500}],"level":"log","timestamp":1789300000125}],
  "event":{"request":{"url":"https://app.acme.workers.dev/board/add?topic=lobby","method":"POST","headers":{}},"response":{"status":200}}}`

// fakeTail serves the tails API and a tail WebSocket that sends events, then (optionally) stays open.
type fakeTail struct {
	mu        sync.Mutex
	deleted   []string
	protocols []string
	events    []string
	hold      bool
}

func (f *fakeTail) server(t *testing.T) *httptest.Server {
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/accounts/acc/workers/scripts/app/tails", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, `{"success":false,"errors":[{"code":10000,"message":"auth"}]}`, 403)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]string{
			"id": "t1", "url": "ws" + strings.TrimPrefix(srv.URL, "http") + "/tail/t1", "expires_at": "2026-09-14T18:00:00Z"}})
	})
	mux.HandleFunc("/accounts/acc/workers/scripts/app/tails/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			f.mu.Lock()
			f.deleted = append(f.deleted, strings.TrimPrefix(r.URL.Path, "/accounts/acc/workers/scripts/app/tails/"))
			f.mu.Unlock()
			w.Write([]byte(`{"success":true,"result":null}`))
		}
	})
	mux.HandleFunc("/tail/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.protocols = r.Header.Values("Sec-WebSocket-Protocol")
		f.mu.Unlock()
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{cftail.Subprotocol}})
		if err != nil {
			return
		}
		defer c.CloseNow()
		for _, e := range f.events {
			c.Write(r.Context(), websocket.MessageText, []byte(e))
		}
		if f.hold {
			c.Read(r.Context()) // until the client closes
			return
		}
		c.Close(websocket.StatusNormalClosure, "")
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestTail(t *testing.T) {
	f := &fakeTail{events: []string{sample, sample}, hold: true}
	srv := f.server(t)
	ready := false
	c := &cftail.Client{Token: "tok", AccountID: "acc", BaseURL: srv.URL, Ready: func() { ready = true }}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var got []cftail.Event
	err := c.Tail(ctx, "app", func(e cftail.Event) {
		if !ready {
			t.Error("event before Ready")
		}
		got = append(got, e)
		if len(got) == 2 {
			cancel() // like Ctrl-C
		}
	})
	if err != nil {
		t.Fatalf("Tail: %v", err)
	}
	if len(got) != 2 || got[0].Event.Request.Method != "POST" || got[0].Event.Response.Status != 200 {
		t.Fatalf("events = %+v", got)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.protocols) != 1 || f.protocols[0] != cftail.Subprotocol {
		t.Errorf("subprotocols = %v, want [%s]", f.protocols, cftail.Subprotocol)
	}
	if len(f.deleted) != 1 || f.deleted[0] != "t1" {
		t.Errorf("deleted tails = %v, want [t1] after cancel", f.deleted)
	}
}

func TestTailStreamEnds(t *testing.T) {
	f := &fakeTail{events: []string{sample}}
	srv := f.server(t)
	c := &cftail.Client{Token: "tok", AccountID: "acc", BaseURL: srv.URL}
	n := 0
	err := c.Tail(context.Background(), "app", func(cftail.Event) { n++ })
	if err == nil || !strings.Contains(err.Error(), "stream") || n != 1 {
		t.Errorf("err = %v, events %d; want a stream error after 1 event", err, n)
	}
	if len(f.deleted) != 1 {
		t.Errorf("tail not deleted after the stream ended: %v", f.deleted)
	}
	if _, err := (&cftail.Client{Token: "bad", AccountID: "acc", BaseURL: srv.URL}).Start(context.Background(), "app"); err == nil {
		t.Errorf("bad token: want an error")
	}
}

func TestFormat(t *testing.T) {
	var e cftail.Event
	if err := json.Unmarshal([]byte(sample), &e); err != nil {
		t.Fatal(err)
	}
	want := "11:46:40.123 POST https://app.acme.workers.dev/board/add?topic=lobby 200 (ok)\n" +
		"  log: publish lobby v3: {\"code\":500}\n" +
		"  exception Error: boom"
	if got := cftail.Format(e); got != want {
		t.Errorf("Format =\n%s\nwant\n%s", got, want)
	}
}
