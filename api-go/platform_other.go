//go:build !(js && wasm)

package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/coder/websocket"

	"github.com/joeblew999/orpc-api/api-go/api"
)

var memory = &api.MemStore{}

// env is one in-memory store and hub for the process.
func env() api.Env {
	return api.Env{
		Var: func(name string) string {
			if name == "APP_NAME" && os.Getenv(name) == "" {
				return "orpc-api-go (go run)"
			}
			return os.Getenv(name)
		},
		Store: func() (api.Store, error) { return memory, nil },
		Hub:   func() (api.Hub, error) { return memory, nil },
	}
}

// serve is the native WebSocket transport, the same job as worker/index.mjs on Cloudflare: the API
// answers an upgrade with a stream of lines (or with an error, passed on as it is), and each line
// is sent as one text frame. When the stream ends, the socket is closed with 1011.
func serve(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			h.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		reader, writer := io.Pipe()
		lines := &lineResponse{header: http.Header{}, status: http.StatusOK, stream: writer, started: make(chan struct{})}
		go func() {
			h.ServeHTTP(lines, r.WithContext(ctx))
			lines.start()
			writer.Close()
		}()
		<-lines.started
		if lines.status != http.StatusOK {
			io.Copy(io.Discard, reader) // until the handler has returned
			for name, values := range lines.header {
				w.Header()[name] = values
			}
			w.WriteHeader(lines.status)
			w.Write(lines.refusal.Bytes())
			return
		}
		socket, err := websocket.Accept(w, r, nil)
		if err != nil {
			reader.Close()
			return
		}
		open := socket.CloseRead(ctx) // done when the client closes
		go func() { <-open.Done(); cancel(); reader.Close() }()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(nil, 1<<20)
		for scanner.Scan() {
			if len(scanner.Bytes()) == 0 {
				continue
			}
			if socket.Write(ctx, websocket.MessageText, scanner.Bytes()) != nil {
				break
			}
		}
		socket.Close(websocket.StatusInternalError, "hub unavailable; reconnect with after")
	})
}

// lineResponse collects what the API answers an upgrade with: a 200 goes into the stream, anything
// else is kept to be sent as the HTTP response.
type lineResponse struct {
	header  http.Header
	status  int
	stream  *io.PipeWriter
	refusal bytes.Buffer
	started chan struct{}
	once    sync.Once
}

func (l *lineResponse) Header() http.Header    { return l.header }
func (l *lineResponse) WriteHeader(status int) { l.status = status }
func (l *lineResponse) Flush()                 {}
func (l *lineResponse) start()                 { l.once.Do(func() { close(l.started) }) }

func (l *lineResponse) Write(p []byte) (int, error) {
	if l.status != http.StatusOK {
		return l.refusal.Write(p)
	}
	l.start()
	return l.stream.Write(p)
}
