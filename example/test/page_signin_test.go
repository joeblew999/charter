package test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/pages"
	"github.com/joeblew999/charter/example/pages/screens"
	notes "github.com/joeblew999/charter/example/sdk/go"
	"github.com/joeblew999/charter/example/sdk/go/option"
)

// Who may do what is one rule for the pages and for the API: writing a note needs someone who may write. A reader
// signs in with a passkey (the browser's virtual authenticator here, a finger or a face otherwise); a program
// carries a token that person made.

// Nobody is offered the way to sign in where the form would be; signed up, they have the form, their note is kept,
// and the header says who they are. Signed out, they are nobody again, and their passkey signs them back in.
func TestSigningInOnThePage(t *testing.T) {
	page := apptest.Browser(t)
	page.Open(at(pages.Home{}))
	page.WaitText("#sign-in-to-write", "Sign in")
	page.SignUp(at(screens.SignInPage{}), "Ada")
	page.Open(at(pages.Home{}))
	page.WaitText("#account-link", "Ada")
	body := "by Ada at " + time.Now().Format(time.RFC3339Nano)
	page.Type(el(pages.Home.Form)+` input[name=body]`, body)
	page.Click(el(pages.Home.Form) + ` button[type=submit]`)
	page.WaitText(el(pages.Home.List), body)

	page.Open(at(screens.AccountPage{}))
	page.WaitText("#signed-in", "Ada")
	page.Click("#sign-out")
	page.WaitText("#sign-in-to-write", "Sign in")
	page.SignIn(at(screens.SignInPage{}))
	page.Open(at(pages.Home{}))
	page.WaitText("#account-link", "Ada")
}

// What is someone's own is nobody else's: the account page sends whoever is not signed in to sign in, and brings
// them back to it afterwards.
func TestTheAccountPageNeedsSomeoneSignedIn(t *testing.T) {
	page := apptest.Browser(t)
	page.Open(at(screens.AccountPage{}))
	if path := page.Path(); path != at(screens.SignInPage{}) {
		t.Fatalf("nobody opening the account page is at %s", path)
	}
	page.Type("#sign-up-name", "Grace")
	page.Click(`#sign-up button[type=submit]`)
	page.WaitText("#signed-in", "Grace")
	if path := page.Path(); path != at(screens.AccountPage{}) {
		t.Errorf("after signing in, at %s", path)
	}
}

// The same rule on both sides, for each kind of caller. Nobody: the page's form is refused, and so is the API's
// createNote. Someone signed in: both are let through. And the token that person makes on their account page is
// them, through the API's own SDK.
func TestThePageAndTheAPIAgreeOnWhoMayWrite(t *testing.T) {
	base := apptest.Base(t)
	form := func(client *http.Client) int {
		res, err := client.PostForm(base+at(pages.PostNote{}), url.Values{"body": {"a note"}})
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return res.StatusCode
	}
	create := func(options ...option.RequestOption) error {
		_, err := sdk(t, options...).Notes.Create(t.Context(), &notes.CreateNotesRequest{Body: "a note"})
		return err
	}
	// Nobody.
	plain := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if code := form(plain); code != http.StatusSeeOther {
		t.Errorf("nobody posting the page's form: HTTP %d, want a redirect to sign in", code)
	}
	if err := create(); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("nobody calling createNote: %v, want 401", err)
	}
	// Someone signed in: a token of theirs, made on the account page.
	page := apptest.Browser(t)
	page.SignUp(at(screens.SignInPage{}), "Lin")
	page.Open(at(screens.AccountPage{}))
	page.Type(el(screens.AccountPage.Tokens)+` input[name=name]`, "a test")
	page.Click(el(screens.AccountPage.Tokens) + ` button[type=submit]`)
	page.WaitText("#token-made", "ch_")
	token := strings.TrimSpace(page.Text("#token-made"))
	if err := create(option.WithToken(token)); err != nil {
		t.Errorf("createNote with a token made on the account page: %v", err)
	}
	// Revoked, it is nobody's.
	page.Click(el(screens.AccountPage.Tokens) + ` li button`)
	page.WaitText(el(screens.AccountPage.Tokens), "Tokens")
	for start := time.Now(); ; time.Sleep(200 * time.Millisecond) {
		if err := create(option.WithToken(token)); err != nil && strings.Contains(err.Error(), "401") {
			break
		}
		if time.Since(start) > 10*time.Second {
			t.Fatal("the token still writes after it was revoked")
		}
	}
}
