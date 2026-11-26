package test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/pages"
	"github.com/joeblew999/charter/example/pages/screens"
	notes "github.com/joeblew999/charter/example/sdk/go"
	"github.com/joeblew999/charter/example/sdk/go/option"
)

// How it's built: each thing Cloudflare gives the Worker doing its job, on a page of its own (pages: Built).

// as is a request by a program with one of the Worker's own tokens: what a form's target accepts as it accepts a
// session, since who may do what is one rule.
func as(t *testing.T, secret, method, path, contentType string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, apptest.Base(t)+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+apptest.Secret(t, secret))
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// soon waits for a page to say something: what a queue's consumer or an email's arrival writes a moment later.
func soon(t *testing.T, what, path, want string) string {
	t.Helper()
	var html string
	for start := time.Now(); time.Since(start) < 30*time.Second; time.Sleep(300 * time.Millisecond) {
		if html = apptest.Get(t, path); strings.Contains(html, want) {
			return html
		}
	}
	t.Fatalf("%s: %s never said %q", what, path, want)
	return ""
}

// The overview lists every job and says where the request came from, and asks no binding to do it: it is the same
// in every language the app has, in that language.
func TestTheOverviewOfHowItIsBuilt(t *testing.T) {
	html := apptest.Get(t, at(pages.BuiltIndex{}))
	for _, want := range []string{"How it&#39;s built", "Attachments", "Activity", "Drafts", "Mail", "Limits and cache", "Outside", `id="facts"`, "charter-example",
		at(pages.AttachmentsPage{}), at(pages.ActivityPage{}), at(pages.DraftsPage{}), at(pages.MailPage{}), at(pages.LimitsPage{}), at(pages.OutsidePage{})} {
		if !strings.Contains(html, want) {
			t.Errorf("the overview has no %s", want)
		}
	}
	if de := apptest.Get(t, "/de"+at(pages.BuiltIndex{})); !strings.Contains(de, "So ist es gebaut") || !strings.Contains(de, "Anhänge") || !strings.Contains(de, `href="/de/built/activity"`) {
		t.Errorf("the overview in German is not German, or leaves it")
	}
}

// A file attached is kept in the bucket and listed from the database: it comes back byte for byte, a reader may
// not take it away, and whoever may moderate may.
func TestAFileIsAttachedAndComesBack(t *testing.T) {
	content := []byte("a file for the bucket, " + time.Now().Format(time.RFC3339Nano))
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "hello.txt")
	part.Write(content)
	form.WriteField("note", "0")
	form.Close()
	res := as(t, "WRITE_TOKEN", "POST", at(pages.AttachFile{}), form.FormDataContentType(), &body)
	res.Body.Close()
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("attaching: HTTP %d", res.StatusCode)
	}
	if res := as(t, "", "POST", at(pages.AttachFile{}), form.FormDataContentType(), strings.NewReader("")); res.StatusCode != http.StatusSeeOther || !strings.Contains(res.Header.Get("Location"), "sign-in") {
		t.Errorf("nobody attaching: HTTP %d to %q, want the way to sign in", res.StatusCode, res.Header.Get("Location"))
	}
	html := apptest.Get(t, at(pages.AttachmentsPage{}))
	i := strings.Index(html, "hello.txt")
	if i < 0 {
		t.Fatalf("the page does not list the file")
	}
	// The newest file's address is the first on the page.
	list := html[strings.Index(html, `id="files"`):]
	href := list[strings.Index(list, `href="`)+6:]
	href = href[:strings.Index(href, `"`)]
	got := apptest.Get(t, href)
	if got != string(content) {
		t.Errorf("the file came back as %q", got)
	}
	remove := href + "/remove"
	if res := as(t, "READ_TOKEN", "POST", remove, "", nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("a reader removing the file: HTTP %d, want 403", res.StatusCode)
	}
	if res := as(t, "ADMIN_TOKEN", "POST", remove, "", nil); res.StatusCode != http.StatusOK {
		t.Errorf("a moderator removing the file: HTTP %d", res.StatusCode)
	}
	if res, err := http.Get(apptest.Base(t) + href); err != nil || res.StatusCode != http.StatusNotFound {
		t.Errorf("the file after it was removed: %v %v", res, err)
	}
}

// A note written is on the queue and done; the queue's consumer records it a moment later, and the open page shows
// it without being asked again.
func TestActivityFollowsTheQueueLive(t *testing.T) {
	page := apptest.Browser(t)
	page.Open(at(pages.ActivityPage{}))
	writer := sdk(t, option.WithToken(apptest.Secret(t, "WRITE_TOKEN")))
	if _, err := writer.Notes.Create(t.Context(), &notes.CreateNotesRequest{Body: "for the activity page"}); err != nil {
		t.Fatal(err)
	}
	page.WaitText(el(pages.ActivityPage.Row)+` li:first-child`, "wrote a note")
}

// The Cron Trigger's task counts the day's notes: fired by hand on a local Worker, its line is on the page, in the
// reader's language with its plural.
func TestTheDaysCountFromTheCronTrigger(t *testing.T) {
	if !apptest.Local() {
		t.Skip("a Cron Trigger is fired by hand on a local Worker only")
	}
	res, err := http.Get(apptest.Base(t) + "/__scheduled?cron=0+6+*+*+*")
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("firing the Cron Trigger: %v %v", res, err)
	}
	res.Body.Close()
	soon(t, "the day's count", at(pages.ActivityPage{}), "in the last day")
	if de := apptest.Get(t, "/de"+at(pages.ActivityPage{})); !strings.Contains(de, "am letzten Tag") {
		t.Errorf("the day's count is not in German on the German page")
	}
}

// What someone types on the notes page is kept as they type, is there on the drafts page, and is gone once posted.
func TestADraftIsKeptUntilItIsPosted(t *testing.T) {
	page := apptest.Browser(t)
	page.SignUp(at(screens.SignInPage{}), "Drafter")
	page.Open(at(pages.Home{}))
	text := "a draft, " + time.Now().Format("150405.000000")
	page.Type(el(pages.Home.Form)+` input[name=body]`, text)
	page.WaitText(el(pages.Home.Saved), "Saved")
	page.Open(at(pages.DraftsPage{}))
	page.WaitText("#draft", text)
	// It is in the form again on the notes page; posted, it is a draft no longer.
	page.Open(at(pages.Home{}))
	if kept := page.Attr(el(pages.Home.Form)+` input[name=body]`, "value"); kept != text {
		t.Errorf("the form has %q, want the draft", kept)
	}
	page.Click(el(pages.Home.Form) + ` button[type=submit]`)
	page.WaitText(el(pages.Home.List), text)
	page.Open(at(pages.DraftsPage{}))
	page.WaitText("#draft", "no draft")
}

// The Worker sends its owner a message and is handed one: the page says to whom and from whom, shortened, and when.
func TestMailOutAndIn(t *testing.T) {
	if !apptest.Local() {
		t.Skip("a local Worker keeps what it sends and can be handed a message; a deployed one sends real email, one an hour")
	}
	if res := as(t, "WRITE_TOKEN", "POST", at(pages.MailSend{}), "", nil); res.StatusCode != http.StatusSeeOther {
		t.Fatalf("sending: HTTP %d", res.StatusCode)
	}
	// The address is a part of the sentence that Intl sets apart, so the two are looked for apart.
	if html := soon(t, "the message sent", at(pages.MailPage{}), "y…@example.com"); !strings.Contains(html, "Sent to") {
		t.Errorf("the page does not say the message was sent")
	}
	// wrangler dev hands the Worker a message posted here, as Email Routing would.
	raw := "From: ada@example.com\r\nTo: site@example.com\r\nSubject: A note for the site\r\nMessage-ID: <note@example.com>\r\n\r\nHello.\r\n"
	res, err := http.Post(apptest.Base(t)+"/cdn-cgi/handler/email?from=ada@example.com&to=site@example.com", "message/rfc822", strings.NewReader(raw))
	if err != nil || res.StatusCode/100 != 2 {
		t.Fatalf("handing the Worker a message: %v %v", res, err)
	}
	res.Body.Close()
	html := soon(t, "the message received", at(pages.MailPage{}), "a…@example.com")
	if !strings.Contains(html, "Received from") || !strings.Contains(html, "A note for the site") {
		t.Errorf("the page does not say what the message was about")
	}
}

// The notes in numbers are worked out once and given from the cache until a minute has passed; a call against the
// write limit is allowed, and says so.
func TestTheCacheAndTheLimit(t *testing.T) {
	first := apptest.Get(t, at(pages.LimitsPage{}))
	if !strings.Contains(first, `id="report"`) || !strings.Contains(first, "100 writes in 60 seconds") {
		t.Fatalf("the page has no report, or no limit:\n%s", first)
	}
	soon(t, "the report from the cache", at(pages.LimitsPage{}), `data-cached="true"`)
	res := as(t, "", "POST", at(pages.LimitUse{}), "", nil)
	said, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(said), `data-allowed="true"`) {
		t.Errorf("one call against the limit: HTTP %d\n%s", res.StatusCode, said)
	}
}

// The Worker asks another site only when asked to: the page itself makes no request, and the button's answer says
// what the other site said and how long it took.
func TestAnotherSiteIsAskedOnlyWhenAsked(t *testing.T) {
	if page := apptest.Get(t, at(pages.OutsidePage{})); strings.Contains(page, `id="outside-said"`) {
		t.Errorf("the page asked another site to be shown")
	}
	res := as(t, "", "POST", at(pages.OutsideFetch{}), "", nil)
	said, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(said), "example.com") || !strings.Contains(string(said), `data-failed="false"`) {
		t.Errorf("fetching another site: HTTP %d\n%s", res.StatusCode, said)
	}
}
