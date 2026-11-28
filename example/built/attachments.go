//go:build js && wasm

package built

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/syumai/workers-go/cloudflare/r2"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/db"
	"github.com/joeblew999/charter/example/env"
)

// R2: a file attached to a note. The bucket keeps the file; the database keeps what is known of it (its name, its
// size, whose it is, which note it goes with), so the page lists them with one query and no walk of the bucket.

// Attachments are the files attached, newest first.
func Attachments(context.Context) ([]Attachment, error) {
	conn, err := env.DB.Open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Attachments(conn, 50)
	out := make([]Attachment, len(rows))
	for i, row := range rows {
		out[i] = Attachment{row}
	}
	return out, err
}

// Attach keeps a file for whoever is asking, with a note of theirs or with none (0).
func Attach(_ context.Context, by charter.Caller, name, kind string, note int64, file []byte) (Attachment, error) {
	if len(file) == 0 || len(file) > MaxFile {
		return Attachment{}, app.Status(http.StatusRequestEntityTooLarge, "a file of up to 1 MB")
	}
	bucket, err := env.BUCKET.Open()
	if err != nil {
		return Attachment{}, err
	}
	conn, err := env.DB.Open()
	if err != nil {
		return Attachment{}, err
	}
	object := "attachments/" + strconv.FormatInt(time.Now().UnixNano(), 36)
	if _, err := bucket.Put(object, io.NopCloser(bytes.NewReader(file)), &r2.PutOptions{HTTPMetadata: r2.HTTPMetadata{ContentType: kind}}); err != nil {
		return Attachment{}, err
	}
	row, err := db.AddAttachment(conn, object, name, kind, int64(len(file)), note, by.Subject, by.Name())
	return Attachment{row}, err
}

// File is an attachment and what the bucket has of it.
func File(_ context.Context, id int64) (Attachment, io.ReadCloser, error) {
	conn, err := env.DB.Open()
	if err != nil {
		return Attachment{}, nil, err
	}
	row, err := db.Attachment(conn, id)
	if err != nil {
		return Attachment{}, nil, app.NotFound("no such file")
	}
	bucket, err := env.BUCKET.Open()
	if err != nil {
		return Attachment{}, nil, err
	}
	object, err := bucket.Get(row.Object)
	if err != nil || object == nil {
		return Attachment{}, nil, app.NotFound("the file is gone")
	}
	return Attachment{row}, io.NopCloser(object.Body), nil
}

// Detach takes a file away, for whoever attached it, or whoever may moderate.
func Detach(_ context.Context, by charter.Caller, id int64) error {
	conn, err := env.DB.Open()
	if err != nil {
		return err
	}
	row, err := db.Attachment(conn, id)
	if err != nil {
		return app.NotFound("no such file")
	}
	if !MayDetach(by, Attachment{row}) {
		return app.Status(http.StatusForbidden, "it is someone else's file")
	}
	bucket, err := env.BUCKET.Open()
	if err != nil {
		return err
	}
	if err := bucket.Delete(row.Object); err != nil {
		return err
	}
	_, err = db.RemoveAttachment(conn, id)
	return err
}

// forget takes away the files someone attached, from the bucket and from the table: what goes with their account.
func forget(_ context.Context, who charter.Person) error {
	conn, err := env.DB.Open()
	if err != nil {
		return err
	}
	rows, err := db.AttachmentsBy(conn, who.ID)
	if err != nil || len(rows) == 0 {
		return err
	}
	bucket, err := env.BUCKET.Open()
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := bucket.Delete(row.Object); err != nil {
			return err
		}
		if _, err := db.RemoveAttachment(conn, row.ID); err != nil {
			return err
		}
	}
	return nil
}

// NotesBy are someone's own notes, newest first: what they may attach a file to.
func NotesBy(_ context.Context, who charter.Caller) ([]db.NotesRow, error) {
	if who.Subject == "" {
		return nil, nil
	}
	conn, err := env.DB.Open()
	if err != nil {
		return nil, err
	}
	return db.NotesBy(conn, who.Subject, 20)
}
