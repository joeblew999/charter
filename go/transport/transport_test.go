//go:build !js

package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// dial opens a WebSocket to a server whose handler answers upgrades the way this package describes.
func dial(t *testing.T, h http.Handler, path string, header http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	srv := httptest.NewServer(Serve(h))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	conn, res, err := websocket.Dial(ctx, strings.Replace(srv.URL, "http", "ws", 1)+path, &websocket.DialOptions{HTTPHeader: header})
	if err == nil {
		t.Cleanup(func() { conn.CloseNow() })
	}
	return conn, res, err
}

func read(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(data)
}

// closed reads until the socket closes and returns the close code.
func closed(t *testing.T, conn *websocket.Conn) websocket.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			var closeError websocket.CloseError
			if !errors.As(err, &closeError) {
				t.Fatalf("the socket did not close cleanly: %v", err)
			}
			return closeError.Code
		}
	}
}

func write(t *testing.T, conn *websocket.Conn, kind websocket.MessageType, data string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, kind, []byte(data)); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestAPlainRequestGoesStraightToTheHandler(t *testing.T) {
	srv := httptest.NewServer(Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "plain") })))
	defer srv.Close()
	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if body, _ := io.ReadAll(res.Body); string(body) != "plain" {
		t.Fatalf("got %q", body)
	}
}

func TestAFeedIsSentLineByLineAndItsEndCloses1011(t *testing.T) {
	conn, _, err := dial(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "\n{\"id\":1}\n\n{\"id\":2}\n")
	}), "/live", nil)
	if err != nil {
		t.Fatal(err)
	}
	if one, two := read(t, conn), read(t, conn); one != `{"id":1}` || two != `{"id":2}` {
		t.Fatalf("frames %q %q", one, two)
	}
	if code := closed(t, conn); code != websocket.StatusInternalError {
		t.Fatalf("closed with %d, want 1011", code)
	}
}

func TestARefusalIsTheHTTPAnswerAndNoSocketOpens(t *testing.T) {
	_, res, err := dial(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, "no token")
	}), "/live", nil)
	if err == nil || res == nil || res.StatusCode != 401 || res.Header.Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("err %v, response %+v", err, res)
	}
	if body, _ := io.ReadAll(res.Body); string(body) != "no token" {
		t.Fatalf("body %q", body)
	}
}

// messages is a channel without a feed that takes messages: it answers each with two lines, and
// refuses the message "bad".
func messages(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set(MessagesHeader, MessagesPost)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || r.Header.Get("Upgrade") != "" || r.Header.Get("Sec-WebSocket-Key") != "" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("a message came as %s with headers %v", r.Method, r.Header)
		}
		if string(body) == "bad" {
			http.Error(w, "refused", http.StatusUnprocessableEntity)
			return
		}
		// What the upgrade carried comes with every message: its query and its headers.
		fmt.Fprintf(w, "%s %s %s\n", body, r.URL.Query().Get("token"), r.Header.Get("Authorization"))
		fmt.Fprintf(w, "%s again\n", body)
	})
}

func TestEachMessageIsAPostAndItsAnswerIsSentAsFrames(t *testing.T) {
	conn, _, err := dial(t, messages(t), "/live?token=q", http.Header{"Authorization": {"Bearer h"}})
	if err != nil {
		t.Fatal(err)
	}
	write(t, conn, websocket.MessageText, "one")
	write(t, conn, websocket.MessageText, "two")
	var got []string
	for range 4 {
		got = append(got, read(t, conn))
	}
	if want := "one q Bearer h|one again|two q Bearer h|two again"; strings.Join(got, "|") != want {
		t.Fatalf("frames %q, want %q", strings.Join(got, "|"), want)
	}
	// No feed (204): the socket stays open until the client closes it.
	if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestARefusedMessageCloses1008AndABinaryOne1003(t *testing.T) {
	conn, _, err := dial(t, messages(t), "/live", nil)
	if err != nil {
		t.Fatal(err)
	}
	write(t, conn, websocket.MessageText, "bad")
	if code := closed(t, conn); code != websocket.StatusPolicyViolation {
		t.Fatalf("a refused message closed with %d, want 1008", code)
	}
	conn, _, err = dial(t, messages(t), "/live", nil)
	if err != nil {
		t.Fatal(err)
	}
	write(t, conn, websocket.MessageBinary, "x")
	if code := closed(t, conn); code != websocket.StatusUnsupportedData {
		t.Fatalf("a binary message closed with %d, want 1003", code)
	}
}

func TestWithoutTheHeaderWhatTheClientSendsIsIgnored(t *testing.T) {
	release := make(chan struct{})
	conn, _, err := dial(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("a %s reached the handler", r.Method)
		}
		io.WriteString(w, "first\n")
		w.(http.Flusher).Flush()
		<-release
		io.WriteString(w, "second\n")
	}), "/live", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, conn); got != "first" {
		t.Fatalf("got %q", got)
	}
	write(t, conn, websocket.MessageText, "ignored")
	time.Sleep(200 * time.Millisecond) // long enough for the adapter to have read it
	close(release)
	if got := read(t, conn); got != "second" {
		t.Fatalf("got %q", got)
	}
}
