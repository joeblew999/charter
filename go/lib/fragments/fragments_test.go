package fragments

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joeblew999/charter/go/follow"
	"github.com/joeblew999/charter/go/hub"
)

type item int64

func (i item) Position() int64 { return int64(i) }

// feed is a log in memory with a hub: add appends to the log and publishes.
type feed struct {
	hub.Memory[item]
	mu  sync.Mutex
	log []item
}

func (f *feed) add(t *testing.T) {
	f.mu.Lock()
	next := item(len(f.log) + 1)
	f.log = append(f.log, next)
	f.mu.Unlock()
	if err := f.Publish(context.Background(), next); err != nil {
		t.Fatal(err)
	}
}

func (f *feed) Since(_ context.Context, after int64, limit int) ([]item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []item
	for _, it := range f.log {
		if int64(it) > after && len(out) < limit {
			out = append(out, it)
		}
	}
	return out, nil
}

func (f *feed) Latest(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return int64(len(f.log)), nil
}

func event(_ context.Context, it item) (string, error) {
	if it == 99 {
		return "", errors.New("cannot render")
	}
	return "event: test\n" + Data("n ", strconv.Itoa(int(it))+"\nline two"), nil
}

// open serves the stream and returns a function that reads its next event (nil at its end).
func open(t *testing.T, stream Stream[item]) func() *string {
	t.Helper()
	srv := httptest.NewServer(stream)
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	if res.Header.Get("Content-Type") != "text/event-stream" || res.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("headers: %v", res.Header)
	}
	events := make(chan *string, 10)
	go func() {
		lines := bufio.NewScanner(res.Body)
		var event strings.Builder
		for lines.Scan() {
			if lines.Text() != "" {
				event.WriteString(lines.Text() + "\n")
				continue
			}
			if text := event.String(); !strings.HasPrefix(text, ":") {
				events <- &text
			}
			event.Reset()
		}
		events <- nil
	}()
	return func() *string {
		select {
		case event := <-events:
			return event
		case <-time.After(5 * time.Second):
			t.Fatal("no event in 5 s")
			return nil
		}
	}
}

// The items after the position, then each new one as it comes: id, then what Event writes.
func TestEveryItemAfterThePositionIsAnEventWithItsID(t *testing.T) {
	f := &feed{}
	f.add(t)
	f.add(t)
	after := int64(1)
	next := open(t, Stream[item]{Feed: f, Options: follow.Options{After: &after}, Event: event})
	if got := next(); got == nil || *got != "id: 2\nevent: test\ndata: n 2\ndata: n line two\n" {
		t.Fatalf("caught up: %v", got)
	}
	f.add(t)
	if got := next(); got == nil || !strings.HasPrefix(*got, "id: 3\n") {
		t.Fatalf("live: %v", got)
	}
}

// A stream ends when For has passed (the browser connects again), and when Event fails.
func TestAStreamEnds(t *testing.T) {
	f := &feed{}
	next := open(t, Stream[item]{Feed: f, Event: event, For: 200 * time.Millisecond})
	if got := next(); got != nil {
		t.Fatalf("an event: %s", *got)
	}
	f.mu.Lock()
	f.log = append(f.log, 98)
	f.mu.Unlock()
	after := int64(97)
	next = open(t, Stream[item]{Feed: f, Options: follow.Options{After: &after}, Event: event})
	f.mu.Lock()
	f.log = append(f.log, 99)
	f.mu.Unlock()
	f.Publish(context.Background(), 99)
	if got := next(); got == nil || !strings.HasPrefix(*got, "id: 98\n") {
		t.Fatalf("98: %v", got)
	}
	if got := next(); got != nil {
		t.Fatalf("after Event failed: %s", *got)
	}
}

// Every line break SSE knows (CRLF, CR, LF) starts another data line with the prefix.
func TestData(t *testing.T) {
	if got, want := Data("elements ", "<li>one\r\ntwo\rthree\nfour</li>"), "data: elements <li>one\ndata: elements two\ndata: elements three\ndata: elements four</li>"; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if got := Data("", "x"); got != "data: x" {
		t.Errorf("got %q", got)
	}
}
