//go:build !(js && wasm)

package transport

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/syumai/workers-go"
)

// Run serves h as a plain HTTP server on :9900 (or $PORT): workers-go's native build.
func Run(h http.Handler) {
	workers.Serve(Serve(h))
}

// Serve is the native WebSocket adapter, the same job as worker/websocket.mjs on Cloudflare: see
// the package comment for what the API answers an upgrade with.
func Serve(h http.Handler) http.Handler {
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
		if lines.status != http.StatusOK && lines.status != http.StatusNoContent {
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
		// Read what the client sends, until it closes (or a message is refused).
		takes := strings.EqualFold(lines.header.Get(MessagesHeader), MessagesPost)
		go func() {
			defer func() { cancel(); reader.Close() }()
			for {
				kind, frame, err := socket.Read(ctx)
				if err != nil {
					return
				}
				if !takes {
					continue
				}
				if kind != websocket.MessageText {
					socket.Close(websocket.StatusUnsupportedData, "text frames only")
					return
				}
				if status := message(ctx, h, r, socket, frame); status < 200 || status > 299 {
					socket.Close(websocket.StatusPolicyViolation, fmt.Sprintf("message refused: HTTP %d", status))
					return
				}
			}
		}()
		if lines.status == http.StatusNoContent {
			<-ctx.Done()
			socket.Close(websocket.StatusNormalClosure, "")
			return
		}
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

// message gives one frame from the client to the API as a POST to the upgrade's URL, and sends the
// lines of the answer as frames. It returns the answer's status.
func message(ctx context.Context, h http.Handler, upgrade *http.Request, socket *websocket.Conn, frame []byte) int {
	r := upgrade.Clone(ctx)
	r.Method, r.Body, r.ContentLength = http.MethodPost, io.NopCloser(bytes.NewReader(frame)), int64(len(frame))
	for name := range r.Header {
		if lower := strings.ToLower(name); lower == "upgrade" || lower == "connection" || strings.HasPrefix(lower, "sec-websocket-") {
			r.Header.Del(name)
		}
	}
	r.Header.Set("Content-Type", "application/json")
	frames := &frameResponse{ctx: ctx, socket: socket, header: http.Header{}, status: http.StatusOK}
	h.ServeHTTP(frames, r)
	return frames.status
}

// frameResponse sends each line of a 2xx answer (it ends with a newline) as one text frame, as it
// is written.
type frameResponse struct {
	ctx     context.Context
	socket  *websocket.Conn
	header  http.Header
	status  int
	pending []byte
}

func (f *frameResponse) Header() http.Header    { return f.header }
func (f *frameResponse) WriteHeader(status int) { f.status = status }
func (f *frameResponse) Flush()                 {}

func (f *frameResponse) Write(p []byte) (int, error) {
	if f.status < 200 || f.status > 299 {
		return len(p), nil
	}
	f.pending = append(f.pending, p...)
	for {
		line, rest, found := bytes.Cut(f.pending, []byte("\n"))
		if !found {
			return len(p), nil
		}
		f.pending = rest
		if len(line) > 0 {
			if err := f.socket.Write(f.ctx, websocket.MessageText, line); err != nil {
				return len(p), err
			}
		}
	}
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
