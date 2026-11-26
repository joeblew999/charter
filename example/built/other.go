//go:build !(js && wasm)

package built

import (
	"context"
	"io"
	"net/http"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/db"
	"github.com/joeblew999/charter/live"
)

// Off a Worker there is none of what Cloudflare gives one: a test of a page renders it with nothing to show, and
// what would use a binding says the Worker is needed.

func Background() {}

func openLog() (live.Log[Activity], error) { return nil, cloud.ErrNotCloudflare }

func Record(context.Context, cloud.DB, string, int64, string, int64) error {
	return cloud.ErrNotCloudflare
}
func Recent(context.Context, int) ([]Activity, int64, error)         { return nil, 0, nil }
func Attachments(context.Context) ([]Attachment, error)              { return nil, nil }
func NotesBy(context.Context, charter.Caller) ([]db.NotesRow, error) { return nil, nil }
func DraftOf(context.Context, string) (Draft, error)                 { return Draft{}, nil }
func SaveDraft(context.Context, string, string) (Draft, error) {
	return Draft{}, cloud.ErrNotCloudflare
}
func MailNow(context.Context) (Mail, error)               { return Mail{}, nil }
func SendMail(context.Context) error                      { return cloud.ErrNotCloudflare }
func UseLimit(string) (Limit, error)                      { return Limit{}, cloud.ErrNotCloudflare }
func ReportNow(*http.Request) (Report, error)             { return Report{}, nil }
func Fetch(context.Context) Outside                       { return Outside{Failed: true} }
func Socket(context.Context) Outside                      { return Outside{Failed: true} }
func Detach(context.Context, charter.Caller, int64) error { return cloud.ErrNotCloudflare }

func Attach(context.Context, charter.Caller, string, string, int64, []byte) (Attachment, error) {
	return Attachment{}, cloud.ErrNotCloudflare
}

func File(context.Context, int64) (Attachment, io.ReadCloser, error) {
	return Attachment{}, nil, cloud.ErrNotCloudflare
}
