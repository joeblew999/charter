package test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	notes "github.com/joeblew999/charter/example/sdk/go"
	"github.com/joeblew999/charter/example/sdk/go/client"
	"github.com/joeblew999/charter/example/sdk/go/option"
)

// The API is asked through its own generated Go SDK (sdk/go, which charter pack -publish writes from the contract):
// a real client, the one an app in another repository would import.

func sdk(t *testing.T, options ...option.RequestOption) *client.Client {
	return client.NewClient(append([]option.RequestOption{option.WithBaseURL(apptest.Base(t))}, options...)...)
}

func TestTheSDKSaysHello(t *testing.T) {
	hello, err := sdk(t).Meta.Hello(context.Background())
	if err != nil || !strings.HasPrefix(hello.Message, "Hello from charter-example") {
		t.Fatalf("hello: %+v, %v", hello, err)
	}
}

// Reading is public; writing needs the write token.
func TestWritingNeedsTheToken(t *testing.T) {
	_, err := sdk(t).Notes.Create(context.Background(), &notes.CreateNotesRequest{Body: "refused"})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("a write without a token: %v, want 401", err)
	}
	if _, err := sdk(t).Notes.List(context.Background(), &notes.ListNotesRequest{}); err != nil {
		t.Fatalf("a read without a token: %v", err)
	}
}

// A note created through the SDK is in the list, and reaches a client that follows the stream: the feed.
func TestTheSDKFollowsTheFeed(t *testing.T) {
	writer := sdk(t, option.WithToken(apptest.Secret(t, "WRITE_TOKEN")))
	first, err := writer.Notes.Create(context.Background(), &notes.CreateNotesRequest{Body: "before the stream"})
	if err != nil {
		t.Fatal(err)
	}
	seconds, after := 20, strconv.FormatInt(int64(first.ID), 10)
	stream, err := sdk(t).Notes.Watch(context.Background(), &notes.WatchNotesRequest{Seconds: &seconds, After: &after})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	body := "seen by the SDK at " + time.Now().Format(time.RFC3339Nano)
	created, err := writer.Notes.Create(context.Background(), &notes.CreateNotesRequest{Body: body})
	if err != nil {
		t.Fatal(err)
	}
	for {
		note, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			t.Fatalf("the stream ended without note %d", created.ID)
		}
		if err != nil {
			t.Fatal(err)
		}
		if note.ID == created.ID && note.Body == body {
			break
		}
	}
	page, err := writer.Notes.List(context.Background(), &notes.ListNotesRequest{})
	if err != nil || len(page.Results) == 0 || page.Results[0].ID != created.ID {
		t.Fatalf("the list's newest is not note %d: %v", created.ID, err)
	}
}

// The same feed reaches an open page: a note posted with the page's form arrives on the page's stream as the HTML
// of its list item, whoever posted it.
func TestThePageFollowsTheFeed(t *testing.T) {
	base := apptest.Base(t)
	home := apptest.Get(t, "/")
	at := strings.Index(home, `hx-sse:connect="`)
	if at < 0 {
		t.Fatal("the home page's list connects to no stream")
	}
	stream := home[at+len(`hx-sse:connect="`):]
	stream = strings.ReplaceAll(stream[:strings.Index(stream, `"`)], "&amp;", "&")
	next := apptest.Events(t, stream, nil)
	body := "seen by the page at " + time.Now().Format(time.RFC3339Nano)
	res, err := http.PostForm(base+"/notes", url.Values{"body": {body}})
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("POST /notes: %v %v", res, err)
	}
	res.Body.Close()
	for {
		if event := next(); strings.Contains(event, body) {
			if !strings.Contains(event, `data: <li id="note-`) {
				t.Fatalf("the event is not the list item's HTML:\n%s", event)
			}
			return
		}
	}
}

// What the form refuses comes back marked, as a 422 htmx swaps in.
func TestTheFormRefusesAnEmptyNote(t *testing.T) {
	res, err := http.PostForm(apptest.Base(t)+"/notes", url.Values{"body": {"  "}})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(b), `aria-invalid="true"`) {
		t.Fatalf("HTTP %d\n%s", res.StatusCode, b)
	}
}
