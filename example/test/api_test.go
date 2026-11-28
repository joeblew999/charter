package test

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/env"
	notes "github.com/joeblew999/charter/example/sdk/go"
	"github.com/joeblew999/charter/example/sdk/go/client"
	"github.com/joeblew999/charter/example/sdk/go/option"
)

// The API is asked through its own generated Go SDK (sdk/go, which charter pack -publish writes from the contract):
// a real client, the one an app in another repository would import.

func sdk(t *testing.T, options ...option.RequestOption) *client.Client {
	return client.NewClient(append([]option.RequestOption{option.WithBaseURL(apptest.Base(t))}, options...)...)
}

// write creates a note through the SDK, and takes it away when the test is done: the Worker a test asks, a
// deployed one too, is left with what it had. One of the Worker's own tokens wrote it, so no person may remove it
// as their own: the operator's token does.
func write(t *testing.T, as *client.Client, body string) *notes.Note {
	t.Helper()
	note, err := as.Notes.Create(t.Context(), &notes.CreateNotesRequest{Body: body})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		operator := sdk(t, option.WithToken(apptest.Secret(t, env.ADMIN_TOKEN)))
		if err := operator.Notes.Delete(context.Background(), &notes.DeleteNotesRequest{ID: note.ID}); err != nil {
			t.Errorf("the note this test wrote (%d) was not taken away: %v", note.ID, err)
		}
	})
	return note
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
	writer := sdk(t, option.WithToken(apptest.Secret(t, env.WRITE_TOKEN)))
	first := write(t, writer, "before the stream")
	seconds, after := 20, strconv.FormatInt(int64(first.ID), 10)
	stream, err := sdk(t).Notes.Watch(context.Background(), &notes.WatchNotesRequest{Seconds: &seconds, After: &after})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	body := "seen by the SDK at " + time.Now().Format(time.RFC3339Nano)
	created := write(t, writer, body)
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
