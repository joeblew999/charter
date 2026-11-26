package pages

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackielii/structpages"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/built"
	"github.com/joeblew999/charter/example/db"
)

// Built is the pages that show what the app is built with (package built): an overview that asks nothing of any
// binding, and a page for each job, which uses what it shows when it is opened and not before. What changes
// something needs someone who may write, as writing a note does; reading is anyone's.
type Built struct {
	index BuiltIndex `route:"GET /{$} Built"`

	attachments AttachmentsPage `route:"GET /attachments Attachments"`
	attach      AttachFile      `route:"POST /attachments AttachFile"`
	file        AttachedFile    `route:"GET /attachments/{file} AttachedFile"`
	detach      DetachFile      `route:"POST /attachments/{file}/remove DetachFile"`

	activity       ActivityPage   `route:"GET /activity Activity"`
	activityStream ActivityStream `route:"GET /activity/stream ActivityStream"`

	notice    NoticePage `route:"GET /notice Notice"`
	setNotice NoticeSet  `route:"POST /notice NoticeSet"`

	mail     MailPage `route:"GET /mail Mail"`
	sendMail MailSend `route:"POST /mail MailSend"`

	limits   LimitsPage `route:"GET /limits Limits"`
	useLimit LimitUse   `route:"POST /limits LimitUse"`

	outside OutsidePage   `route:"GET /outside Outside"`
	fetch   OutsideFetch  `route:"POST /outside/fetch OutsideFetch"`
	socket  OutsideSocket `route:"POST /outside/socket OutsideSocket"`
}

// BuiltIndex is the overview: the pages, and what Cloudflare says of the reader's own request. It asks no binding,
// so it is as fast as a page gets.
type BuiltIndex struct{}

func (BuiltIndex) Props(r *http.Request) (built.Facts, error) { return built.FactsOf(r), nil }

// AttachmentsPage is the files attached to notes (R2).
type AttachmentsPage struct{}

// AttachmentsView is what it shows: the files, the reader, and the reader's own notes, which a file may go with.
type AttachmentsView struct {
	Files  []built.Attachment
	Caller charter.Caller
	Notes  []db.NotesRow
}

func (AttachmentsPage) Props(r *http.Request, caller charter.Caller) (AttachmentsView, error) {
	files, err := built.Attachments(r.Context())
	if err != nil {
		return AttachmentsView{}, err
	}
	notes, err := built.NotesBy(r.Context(), caller)
	return AttachmentsView{Files: files, Caller: caller, Notes: notes}, err
}

// AttachFile is the form's target: it keeps the file and goes back to the page.
type AttachFile struct{}

func (AttachFile) Needs() []string { return []string{"write"} }

func (AttachFile) ServeHTTP(w http.ResponseWriter, r *http.Request, caller charter.Caller) error {
	r.Body = http.MaxBytesReader(w, r.Body, built.MaxFile+64<<10)
	file, header, err := r.FormFile("file")
	if err != nil {
		return app.Status(http.StatusRequestEntityTooLarge, "a file of up to 1 MB")
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	note, _ := strconv.ParseInt(r.FormValue("note"), 10, 64)
	kind := header.Header.Get("Content-Type")
	if kind == "" {
		kind = "application/octet-stream"
	}
	if _, err := built.Attach(r.Context(), caller, header.Filename, kind, note, content); err != nil {
		return err
	}
	back, err := structpages.URLFor(r.Context(), AttachmentsPage{})
	if err != nil {
		return err
	}
	return app.Redirect{To: back}
}

// AttachedFile is a file itself, from the bucket: bytes, not a page.
type AttachedFile struct{}

func (AttachedFile) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("file"), 10, 64)
	about, file, err := built.File(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	// As a download, with no type a browser would run: the file is whatever someone attached.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(about.Name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	io.Copy(w, file)
}

// DetachFile is a file's remove button: it answers with nothing, which is what the row becomes.
type DetachFile struct{}

func (DetachFile) Needs() []string { return []string{"write"} }

func (DetachFile) ServeHTTP(w http.ResponseWriter, r *http.Request, caller charter.Caller) error {
	id, err := strconv.ParseInt(r.PathValue("file"), 10, 64)
	if err != nil {
		return app.NotFound("no such file")
	}
	return built.Detach(r.Context(), caller, id)
}

// ActivityPage is what happened (Queues, a Cron Trigger, D1), followed live (a Durable Object).
type ActivityPage struct{}

// ActivityView is what it shows: the latest, and the position its stream starts after.
type ActivityView struct {
	Latest []built.Activity
	After  int64
}

func (ActivityPage) Props(r *http.Request) (ActivityView, error) {
	latest, after, err := built.Recent(r.Context(), 30)
	return ActivityView{latest, after}, err
}

// ActivityStream is the page's side of the feed: each thing that happens, as the page's own row.
type ActivityStream struct{}

func (ActivityStream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	built.Feed.Fragments(w, r, func(ctx context.Context, a built.Activity) (string, error) {
		var html bytes.Buffer
		err := ActivityPage{}.Row(a).Render(ctx, &html)
		return strings.TrimSpace(html.String()), err
	})
}

// NoticePage is the line for everyone (KV), and the form that sets it.
type NoticePage struct{}

// NoticeView is the notice, and whether the reader may set it.
type NoticeView struct {
	Notice built.Notice
	MaySet bool
}

func (NoticePage) Props(r *http.Request, caller charter.Caller) (NoticeView, error) {
	notice, err := built.NoticeNow(r.Context())
	return NoticeView{notice, caller.May(NoticeSet{}.Needs()...)}, err
}

// NoticeSet shows a line to everyone, and goes back to the page.
type NoticeSet struct{}

// Needs is whoever may moderate: what everyone sees is theirs to say.
func (NoticeSet) Needs() []string { return []string{"moderate"} }

func (NoticeSet) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	if _, err := built.SetNotice(r.Context(), r.PostFormValue("notice")); err != nil {
		return err
	}
	back, err := structpages.URLFor(r.Context(), NoticePage{})
	if err != nil {
		return err
	}
	return app.Redirect{To: back}
}

// MailPage is the last message sent and the last received (email).
type MailPage struct{}

// MailView is those, and whether the reader may send one.
type MailView struct {
	Mail    built.Mail
	MaySend bool
}

func (MailPage) Props(r *http.Request, caller charter.Caller) (MailView, error) {
	mail, err := built.MailNow(r.Context())
	return MailView{mail, caller.May(MailSend{}.Needs()...)}, err
}

// MailSend sends one, and goes back to the page.
type MailSend struct{}

func (MailSend) Needs() []string { return []string{"write"} }

func (MailSend) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	if err := built.SendMail(r.Context()); err != nil {
		return err
	}
	back, err := structpages.URLFor(r.Context(), MailPage{})
	if err != nil {
		return err
	}
	return app.Redirect{To: back}
}

// LimitsPage is the write limit, and an answer kept in the cache.
type LimitsPage struct{}

func (LimitsPage) Props(r *http.Request) (built.Report, built.Limit, error) {
	report, err := built.ReportNow(r)
	return report, built.Limit{Calls: built.LimitCalls, Seconds: built.LimitSeconds}, err
}

// LimitUse makes one call against the limit, as the reader, and answers with whether it was within it.
type LimitUse struct{}

func (LimitUse) ServeHTTP(w http.ResponseWriter, r *http.Request, caller charter.Caller) error {
	key := caller.Subject
	if key == "" {
		key = r.Header.Get("CF-Connecting-IP")
	}
	limit, err := built.UseLimit("page:" + key)
	if err != nil {
		return err
	}
	return structpages.RenderComponent(LimitsPage{}.Limit(limit))
}

// OutsidePage is the two requests the Worker makes to another site, each when its button is pressed.
type OutsidePage struct{}

// OutsideFetch and OutsideSocket make them, and answer with what the other site said.
type (
	OutsideFetch  struct{}
	OutsideSocket struct{}
)

func (OutsideFetch) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return structpages.RenderComponent(OutsidePage{}.Said(built.Fetch(r.Context())))
}

func (OutsideSocket) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return structpages.RenderComponent(OutsidePage{}.Said(built.Socket(r.Context())))
}
