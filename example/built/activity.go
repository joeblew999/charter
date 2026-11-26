//go:build js && wasm

package built

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"time"

	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/cron"
	"github.com/syumai/workers-go/cloudflare/queues"

	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/db"
)

// What happened, kept by the app itself. Three things of Cloudflare's each do their part:
//
//   - a Queue: writing a note puts a message on it and is done; a consumer, a moment later, records that the note
//     was written. Whoever wrote the note did not wait for that.
//   - a Cron Trigger: once a day its task records how many notes the day had.
//   - a Durable Object: the page that shows all this follows it live (Feed), woken as each row is written.

// Background registers what the Worker does that is no request: the queue's consumer, the Cron Trigger's task and
// the consumer of email. And it has each note written told to the queue. main calls it before it serves.
func Background() {
	api.OnPost = queue
	api.OnLeave = forget
	queues.ConsumeNonBlock(consume)
	cron.ScheduleTaskNonBlock(summarise)
	onEmail()
}

// written is the queue's message for a note.
type written struct {
	Note   int64  `json:"note"`
	Author string `json:"author"`
}

// queue puts a note on the queue, after the answer has left: the writer's request does not wait for it.
func queue(_ context.Context, note api.Note) {
	cloudflare.WaitUntil(func() {
		producer, err := queues.NewProducer("QUEUE")
		if err == nil {
			// As text: the queue's own JSON takes JavaScript values, which a Go struct is not.
			message, _ := json.Marshal(written{note.ID, note.Author})
			err = producer.SendText(string(message))
		}
		if err != nil {
			log.Printf("built: a note was not queued: %v", err)
		}
	})
}

// consume records each note the queue hands over.
func consume(batch *queues.MessageBatch) error {
	conn, err := cloud.Open("DB")
	if err != nil {
		return err
	}
	for _, m := range batch.Messages {
		body, err := m.StringBody()
		if err != nil {
			return err
		}
		var note written
		if err := json.Unmarshal([]byte(body), &note); err != nil {
			m.Ack() // not one of ours: it would never become one
			continue
		}
		// A note that was taken away before the queue got to it leaves nothing to record.
		if _, err := db.Note(conn, note.Note); err != nil {
			m.Ack()
			continue
		}
		if err := Record(context.Background(), conn, NoteWritten, note.Note, note.Author, 0); err != nil {
			return err
		}
		m.Ack()
	}
	return nil
}

// summarise is the Cron Trigger's task: how many notes the last day had.
func summarise(ctx context.Context) error {
	conn, err := cloud.Open("DB")
	if err != nil {
		return err
	}
	n, err := db.NotesSince(conn, time.Now().UTC().Add(-24*time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		return err
	}
	return Record(ctx, conn, Summary, 0, "", n)
}

// Record writes that something happened, and wakes whoever follows.
func Record(ctx context.Context, conn cloud.DB, kind string, note int64, author string, notes int64) error {
	row, err := db.RecordActivity(conn, kind, note, author, notes)
	if err != nil {
		return err
	}
	Feed.Publish(ctx, Activity{row})
	return nil
}

// Recent is what happened, newest first, and the position of the newest: where a page's stream starts.
func Recent(_ context.Context, limit int) ([]Activity, int64, error) {
	conn, err := cloud.Open("DB")
	if err != nil {
		return nil, 0, err
	}
	rows, err := db.ActivityBefore(conn, math.MaxInt64, int64(limit))
	out := make([]Activity, len(rows))
	for i, row := range rows {
		out[i] = Activity{row}
	}
	var after int64
	if len(out) > 0 {
		after = out[0].ID
	}
	return out, after, err
}

// activityLog is the table as the feed's log.
type activityLog struct{ db cloud.DB }

func openLog() (activityLog, error) {
	conn, err := cloud.Open("DB")
	return activityLog{conn}, err
}

func (l activityLog) Since(_ context.Context, after int64, limit int) ([]Activity, error) {
	rows, err := db.ActivityAfter(l.db, after, int64(limit))
	out := make([]Activity, len(rows))
	for i, row := range rows {
		out[i] = Activity{row}
	}
	return out, err
}

func (l activityLog) Latest(context.Context) (int64, error) { return db.NewestActivity(l.db) }
