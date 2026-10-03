package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"
)

func init() {
	commands["tail"] = command{"[-worker <name>] [-for <duration>]",
		"REMOTE, read-only: stream the deployed Worker's live logs (each request, its console output and exceptions) until Ctrl-C or -for ends. The Worker: API_URL's, unless -worker names another. Needs CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID (the environment, or fnox)", tail}
}

// tail streams a Worker's live logs with Cloudflare's tail API, without wrangler: it starts a tail,
// reads its WebSocket and deletes the tail when done (Cloudflare allows 10 per Worker, and samples
// a busy one).
//
//	POST   /accounts/{account}/workers/scripts/{worker}/tails       → {id, url, expires_at}
//	DELETE /accounts/{account}/workers/scripts/{worker}/tails/{id}
//
// API: https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/tail/
// Events: https://developers.cloudflare.com/workers/observability/logs/real-time-logs/
func tail(args []string) error {
	var worker string
	var duration time.Duration
	flags("tail", args, func(f *flag.FlagSet) {
		f.StringVar(&worker, "worker", "", "the Worker's name (default: the project's, from API_URL)")
		f.DurationVar(&duration, "for", 0, "stop after this long (default: at Ctrl-C)")
	})
	if worker == "" {
		var err error
		if _, worker, err = deployed(); err != nil {
			return err
		}
	}
	cf, err := newCloudflare()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}
	return tailWorker(ctx, cf, worker, func() {
		fmt.Fprintf(os.Stderr, "tailing %s (Ctrl-C to stop); events start a second or two after this\n", worker)
	}, func(e tailEvent) { fmt.Println(e) })
}

// tailWorker starts a tail on worker, calls ready once its WebSocket is open and each for every event
// until ctx is done (no error) or the stream ends, then deletes the tail.
func tailWorker(ctx context.Context, cf cloudflare, worker string, ready func(), each func(tailEvent)) error {
	var session struct{ ID, URL string }
	path := "/workers/scripts/" + url.PathEscape(worker) + "/tails"
	if err := cf.call("POST", path, map[string]any{}, &session); err != nil {
		return err
	}
	if session.ID == "" || session.URL == "" {
		return errors.New("tail: Cloudflare started a tail with no id or url")
	}
	defer func() {
		// ctx is usually done by now (Ctrl-C); the delete goes anyway, and a tail left behind expires.
		if err := cf.call("DELETE", path+"/"+session.ID, nil, nil); err != nil {
			fmt.Fprintln(os.Stderr, "charter: tail: could not delete the tail, it expires by itself:", err)
		}
	}()
	conn, err := dialWebSocket(ctx, session.URL, tailProtocol)
	if err != nil {
		return fmt.Errorf("tail: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if ready != nil {
		ready()
	}
	for {
		message, err := conn.read()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("tail: the stream ended: %w", err)
		}
		var e tailEvent
		if err := json.Unmarshal(message, &e); err != nil {
			return fmt.Errorf("tail: an event that is not JSON (%.200s): %w", message, err)
		}
		each(e)
	}
}

// tailProtocol is the tail WebSocket's subprotocol. Cloudflare's docs do not name it; it is what
// wrangler tail asks for.
const tailProtocol = "trace-v1"

// tailEvent is one invocation of the Worker as the tail reports it.
type tailEvent struct {
	Outcome        string `json:"outcome"` // "ok", "exception", "exceededCpu", "canceled", ...
	EventTimestamp int64  `json:"eventTimestamp"`
	Exceptions     []struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"exceptions"`
	Logs []struct {
		Message []any  `json:"message"` // the console call's arguments
		Level   string `json:"level"`
	} `json:"logs"`
	Event struct {
		Request *struct {
			URL    string `json:"url"`
			Method string `json:"method"`
		} `json:"request"`
		Response *struct {
			Status int `json:"status"`
		} `json:"response"`
		Cron string `json:"cron"`
	} `json:"event"`
}

// String is one line for the invocation (UTC time, request or cron, status, outcome), then one
// indented line per console call and per exception.
func (e tailEvent) String() string {
	var b strings.Builder
	b.WriteString(time.UnixMilli(e.EventTimestamp).UTC().Format("15:04:05.000"))
	switch r := e.Event.Request; {
	case r != nil:
		fmt.Fprintf(&b, " %s %s", r.Method, r.URL)
		if e.Event.Response != nil {
			fmt.Fprintf(&b, " %d", e.Event.Response.Status)
		}
	case e.Event.Cron != "":
		fmt.Fprintf(&b, " cron %s", e.Event.Cron)
	}
	fmt.Fprintf(&b, " (%s)", e.Outcome)
	for _, l := range e.Logs {
		parts := make([]string, len(l.Message))
		for i, m := range l.Message {
			if s, ok := m.(string); ok {
				parts[i] = s
			} else {
				j, _ := json.Marshal(m)
				parts[i] = string(j)
			}
		}
		fmt.Fprintf(&b, "\n  %s: %s", l.Level, strings.Join(parts, " "))
	}
	for _, x := range e.Exceptions {
		fmt.Fprintf(&b, "\n  exception %s: %s", x.Name, x.Message)
	}
	return b.String()
}

// webSocket is the client end of a WebSocket (RFC 6455), as much as tail needs: it reads messages,
// answers pings and closes.
type webSocket struct {
	conn   net.Conn
	reader *bufio.Reader
}

// dialWebSocket opens a WebSocket to a ws:// or wss:// URL with one subprotocol.
func dialWebSocket(ctx context.Context, address, protocol string) (*webSocket, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	host := u.Host
	var conn net.Conn
	switch u.Scheme {
	case "wss":
		if u.Port() == "" {
			host += ":443"
		}
		conn, err = (&tls.Dialer{Config: &tls.Config{ServerName: u.Hostname()}}).DialContext(ctx, "tcp", host)
	case "ws":
		if u.Port() == "" {
			host += ":80"
		}
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", host)
	default:
		return nil, fmt.Errorf("%s is not a WebSocket URL", u.Redacted())
	}
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, 16)
	rand.Read(nonce)
	key := base64.StdEncoding.EncodeToString(nonce)
	req := &http.Request{Method: "GET", URL: u, Host: u.Host, Header: http.Header{
		"Upgrade":                {"websocket"},
		"Connection":             {"Upgrade"},
		"Sec-WebSocket-Key":      {key},
		"Sec-WebSocket-Version":  {"13"},
		"Sec-WebSocket-Protocol": {protocol},
	}}
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	reader := bufio.NewReader(conn)
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}
	res, err := http.ReadResponse(reader, req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	res.Body.Close()
	conn.SetDeadline(time.Time{})
	if res.StatusCode != http.StatusSwitchingProtocols || res.Header.Get("Sec-WebSocket-Accept") != acceptKey(key) {
		conn.Close()
		return nil, fmt.Errorf("the WebSocket handshake failed: HTTP %s", res.Status)
	}
	return &webSocket{conn, reader}, nil
}

// acceptKey is what the server must answer to a Sec-WebSocket-Key.
func acceptKey(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// read is the next text or binary message, put together from its frames.
func (ws *webSocket) read() ([]byte, error) {
	var message []byte
	for {
		var head [2]byte
		if _, err := io.ReadFull(ws.reader, head[:]); err != nil {
			return nil, err
		}
		fin, opcode, masked := head[0]&0x80 != 0, head[0]&0x0f, head[1]&0x80 != 0
		size := uint64(head[1] & 0x7f)
		switch size {
		case 126:
			var n [2]byte
			if _, err := io.ReadFull(ws.reader, n[:]); err != nil {
				return nil, err
			}
			size = uint64(binary.BigEndian.Uint16(n[:]))
		case 127:
			var n [8]byte
			if _, err := io.ReadFull(ws.reader, n[:]); err != nil {
				return nil, err
			}
			size = binary.BigEndian.Uint64(n[:])
		}
		if size > 64<<20 {
			return nil, fmt.Errorf("a WebSocket frame of %d bytes", size)
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(ws.reader, mask[:]); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(ws.reader, payload); err != nil {
			return nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		switch opcode {
		case 0x8: // close
			ws.write(0x8, payload)
			return nil, errors.New("the server closed the WebSocket")
		case 0x9: // ping
			if err := ws.write(0xa, payload); err != nil {
				return nil, err
			}
			continue
		case 0xa: // pong
			continue
		}
		message = append(message, payload...)
		if fin {
			return message, nil
		}
	}
}

// write sends one frame, masked as a client's must be.
func (ws *webSocket) write(opcode byte, payload []byte) error {
	frame := []byte{0x80 | opcode}
	switch n := len(payload); {
	case n < 126:
		frame = append(frame, 0x80|byte(n))
	case n < 1<<16:
		frame = append(frame, 0x80|126, byte(n>>8), byte(n))
	default:
		frame = append(frame, 0x80|127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(n))
	}
	var mask [4]byte
	rand.Read(mask[:])
	frame = append(frame, mask[:]...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	_, err := ws.conn.Write(frame)
	return err
}

// Close says goodbye (best effort) and closes the connection.
func (ws *webSocket) Close() error {
	ws.conn.SetWriteDeadline(time.Now().Add(time.Second))
	ws.write(0x8, []byte{0x03, 0xe8}) // 1000: normal closure
	return ws.conn.Close()
}
