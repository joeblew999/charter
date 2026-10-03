package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const tailSample = `{"outcome":"ok","scriptName":"app","eventTimestamp":1789300000123,
  "exceptions":[{"name":"Error","message":"boom","timestamp":1789300000124}],
  "logs":[{"message":["publish lobby v3:",{"code":500}],"level":"log","timestamp":1789300000125}],
  "event":{"request":{"url":"https://app.acme.workers.dev/board/add?topic=lobby","method":"POST","headers":{}},"response":{"status":200}}}`

// fakeTail serves Cloudflare's tails API and a tail WebSocket that pings, sends its events (the
// first in two fragments, one of them past 125 bytes), then stays open or ends.
type fakeTail struct {
	mu        sync.Mutex
	protocols []string
	pongs     int
	deleted   []string
	events    []string
	hold      bool
}

func (f *fakeTail) start(t *testing.T) cloudflare {
	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("POST /accounts/acc/workers/scripts/app/tails", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(403)
			w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"auth"}]}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]string{
			"id": "t1", "url": "ws" + strings.TrimPrefix(server.URL, "http") + "/tail/t1", "expires_at": "2026-10-03T18:00:00Z"}})
	})
	mux.HandleFunc("DELETE /accounts/acc/workers/scripts/app/tails/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.deleted = append(f.deleted, r.PathValue("id"))
		f.mu.Unlock()
		w.Write([]byte(`{"success":true,"result":null}`))
	})
	mux.HandleFunc("GET /tail/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.protocols = r.Header.Values("Sec-WebSocket-Protocol")
		f.mu.Unlock()
		conn, rw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Protocol: " + tailProtocol + "\r\nSec-WebSocket-Accept: " + acceptKey(r.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n")
		frame := func(head byte, payload string) {
			rw.WriteByte(head)
			if n := len(payload); n < 126 {
				rw.WriteByte(byte(n))
			} else {
				rw.WriteByte(126)
				rw.Write(binary.BigEndian.AppendUint16(nil, uint16(n)))
			}
			rw.WriteString(payload)
		}
		frame(0x89, "hi") // ping
		for i, e := range f.events {
			if i == 0 {
				frame(0x01, e[:10]) // text, not final
				frame(0x80, e[10:]) // continuation, final
			} else {
				frame(0x81, e)
			}
		}
		rw.Flush()
		// The pong comes back masked: 2 bytes of header, 4 of mask, 2 of payload.
		pong := make([]byte, 8)
		if _, err := io.ReadFull(rw, pong); err == nil && pong[0] == 0x8a && pong[1] == 0x82 {
			f.mu.Lock()
			f.pongs++
			f.mu.Unlock()
		}
		if f.hold {
			io.Copy(io.Discard, rw) // until the client closes
		}
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	api := cloudflareAPI
	cloudflareAPI = server.URL
	t.Cleanup(func() { cloudflareAPI = api })
	return cloudflare{"tok", "acc"}
}

func TestTail(t *testing.T) {
	f := &fakeTail{events: []string{tailSample, tailSample}, hold: true}
	cf := f.start(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ready := false
	var got []tailEvent
	err := tailWorker(ctx, cf, "app", func() { ready = true }, func(e tailEvent) {
		if !ready {
			t.Error("an event before ready")
		}
		if got = append(got, e); len(got) == 2 {
			cancel() // as Ctrl-C does
		}
	})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(got) != 2 || got[0].Event.Request.Method != "POST" || got[0].Event.Response.Status != 200 {
		t.Fatalf("events = %+v", got)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.protocols) != 1 || f.protocols[0] != tailProtocol {
		t.Errorf("subprotocols %v, want [%s]", f.protocols, tailProtocol)
	}
	if f.pongs != 1 {
		t.Errorf("the ping got %d pongs, want 1", f.pongs)
	}
	if len(f.deleted) != 1 || f.deleted[0] != "t1" {
		t.Errorf("deleted tails %v, want [t1]", f.deleted)
	}
}

func TestTailStreamEnds(t *testing.T) {
	f := &fakeTail{events: []string{tailSample}}
	cf := f.start(t)
	n := 0
	err := tailWorker(context.Background(), cf, "app", nil, func(tailEvent) { n++ })
	if err == nil || !strings.Contains(err.Error(), "stream ended") || n != 1 {
		t.Errorf("err %v after %d events; want the stream's end after 1", err, n)
	}
	if len(f.deleted) != 1 {
		t.Errorf("the tail was not deleted after the stream ended: %v", f.deleted)
	}
	if err := tailWorker(context.Background(), cloudflare{"bad", "acc"}, "app", nil, nil); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("a bad token: %v, want HTTP 403", err)
	}
}

func TestTailEventString(t *testing.T) {
	var e tailEvent
	if err := json.Unmarshal([]byte(tailSample), &e); err != nil {
		t.Fatal(err)
	}
	want := "11:46:40.123 POST https://app.acme.workers.dev/board/add?topic=lobby 200 (ok)\n" +
		"  log: publish lobby v3: {\"code\":500}\n" +
		"  exception Error: boom"
	if got := e.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
