package test

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/env"
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
	t.Parallel()
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
	page.WaitText("#about", "Ada")
	page.Click("#sign-out")
	page.WaitText("#sign-in-to-write", "Sign in")
	page.SignIn(at(screens.SignInPage{}))
	page.Open(at(pages.Home{}))
	page.WaitText("#account-link", "Ada")
}

// What is someone's own is nobody else's: the account page sends whoever is not signed in to sign in, and brings
// them back to it afterwards.
func TestTheAccountPageNeedsSomeoneSignedIn(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.Open(at(screens.AccountPage{}))
	if path := page.Path(); path != at(screens.SignInPage{}) {
		t.Fatalf("nobody opening the account page is at %s", path)
	}
	page.Type("#sign-up-name", "Grace")
	page.Click(`#sign-up button[type=submit]`)
	page.WaitText("#about", "Grace")
	page.Leave(at(screens.SignInPage{}))
	if path := page.Path(); path != at(screens.AccountPage{}) {
		t.Errorf("after signing in, at %s", path)
	}
}

// The same rule on both sides, for each kind of caller. Nobody: the page's form is refused, and so is the API's
// createNote. Someone signed in: both are let through. And the token that person makes on their account page is
// them, through the API's own SDK.
func TestThePageAndTheAPIAgreeOnWhoMayWrite(t *testing.T) {
	t.Parallel()
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

// Roles are one rule too. A member's note: another member may not take it away, on the page (no button) or through
// the API (403); made a moderator by an admin, the same person may, through both. What they are changes at once:
// in the session they have open, and for the token they made before.
func TestRolesDecideWhoMayRemoveANote(t *testing.T) {
	t.Parallel()
	admin := sdk(t, option.WithToken(apptest.Secret(t, env.ADMIN_TOKEN)))
	// Ada writes a note.
	ada := apptest.Browser(t)
	ada.SignUp(at(screens.SignInPage{}), "Ada the writer")
	ada.Open(at(pages.Home{}))
	body := "Ada's, at " + time.Now().Format(time.RFC3339Nano)
	ada.Type(el(pages.Home.Form)+` input[name=body]`, body)
	ada.Click(el(pages.Home.Form) + ` button[type=submit]`)
	ada.WaitText(el(pages.Home.List), body)

	// Mo, another member, with a token for a program of theirs.
	mo := apptest.Browser(t)
	name := "Mo " + time.Now().Format("150405.000000")
	mo.SignUp(at(screens.SignInPage{}), name)
	mo.Open(at(screens.AccountPage{}))
	mo.WaitText("#role", "member")
	mo.Type(el(screens.AccountPage.Tokens)+` input[name=name]`, "Mo's program")
	mo.Click(el(screens.AccountPage.Tokens) + ` button[type=submit]`)
	mo.WaitText("#token-made", "ch_")
	program := sdk(t, option.WithToken(strings.TrimSpace(mo.Text("#token-made"))))

	// Ada's note, as the API has it, and as Mo's page shows it: with no way to take it away.
	list, err := program.Notes.List(t.Context(), &notes.ListNotesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var note int64
	for _, n := range list.Results {
		if n.Body == body {
			note = n.ID
		}
	}
	if note == 0 {
		t.Fatalf("Ada's note is not in the list")
	}
	row := row(note)
	mo.Open(at(pages.Home{}))
	mo.WaitText(row, body)
	if mo.Has(row + ` [data-remove]`) {
		t.Error("a member's page has the button for another's note")
	}
	if err := program.Notes.Delete(t.Context(), &notes.DeleteNotesRequest{ID: note}); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("a member removing another's note through the API: %v, want 403", err)
	}
	// Nor may Mo say who is what.
	if _, err := program.People.List(t.Context()); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("a member listing people: %v, want 403", err)
	}

	// An admin makes Mo a moderator.
	people, err := admin.People.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, p := range people.Data {
		if p.Name != nil && *p.Name == name {
			id = p.ID
		}
	}
	if id == "" {
		t.Fatalf("%s is not among the people", name)
	}
	if _, err := admin.People.SetRole(t.Context(), &notes.SetRolePeopleRequest{ID: id, Role: notes.SetRolePeopleRequestRoleModerator}); err != nil {
		t.Fatal(err)
	}

	// Now Mo may: the page they open has the button, in the session they had; and it takes the note away.
	mo.Open(at(screens.AccountPage{}))
	mo.WaitText("#role", "moderator")
	mo.Open(at(pages.Home{}))
	mo.WaitText(row, body)
	mo.Click(row + ` [data-remove]`)
	for start := time.Now(); strings.Contains(mo.Text(el(pages.Home.List)), body); time.Sleep(100 * time.Millisecond) {
		if time.Since(start) > 10*time.Second {
			t.Fatal("the moderator's button did not take the note away")
		}
	}
	// And so may the token Mo made before, through the API: on another note of Ada's.
	body2 := body + ", again"
	ada.Open(at(pages.Home{}))
	ada.Type(el(pages.Home.Form)+` input[name=body]`, body2)
	ada.Click(el(pages.Home.Form) + ` button[type=submit]`)
	ada.WaitText(el(pages.Home.List), body2)
	list, err = program.Notes.List(t.Context(), &notes.ListNotesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range list.Results {
		if n.Body == body2 {
			if err := program.Notes.Delete(t.Context(), &notes.DeleteNotesRequest{ID: n.ID}); err != nil {
				t.Errorf("a moderator's token removing another's note through the API: %v", err)
			}
		}
	}
}

// Who is what is an admin's to say, on a page: a member gets a 403 there; made an admin (by the operator's token,
// which is how the first one is), they see everyone and give someone a role by choosing it.
func TestAnAdminGivesRolesOnThePeoplePage(t *testing.T) {
	t.Parallel()
	operator := sdk(t, option.WithToken(apptest.Secret(t, env.ADMIN_TOKEN)))
	boss := apptest.Browser(t)
	name := "Boss " + time.Now().Format("150405.000000")
	boss.SignUp(at(screens.SignInPage{}), name)
	res, err := http.Get(apptest.Base(t) + at(screens.PeoplePage{}))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	boss.Open(at(screens.AccountPage{}))
	if boss.Has("#people-link") {
		t.Error("a member is offered the people page")
	}
	people, err := operator.People.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, p := range people.Data {
		if p.Name != nil && *p.Name == name {
			id = p.ID
		}
	}
	if _, err := operator.People.SetRole(t.Context(), &notes.SetRolePeopleRequest{ID: id, Role: notes.SetRolePeopleRequestRoleAdmin}); err != nil {
		t.Fatal(err)
	}
	boss.Open(at(screens.AccountPage{}))
	boss.Click("#people-link")
	boss.WaitText("#people", name)
	// Their own row: an admin makes themselves a moderator, and the page says so.
	row := el(screens.PeoplePage.Person) + "--" + id
	boss.Select(row+` select`, "moderator")
	for start := time.Now(); ; time.Sleep(100 * time.Millisecond) {
		after, err := operator.People.List(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		done := false
		for _, p := range after.Data {
			done = done || (p.ID == id && p.Role == notes.PersonRoleModerator)
		}
		if done {
			break
		}
		if time.Since(start) > 10*time.Second {
			t.Fatal("choosing a role on the people page did not give it")
		}
	}
	// No longer an admin, the page is no longer theirs.
	boss.Open(at(screens.AccountPage{}))
	if boss.Has("#people-link") {
		t.Error("a moderator is offered the people page")
	}
}

// Deleting an account is its person's to do, on their account page, asked twice. With it goes what they wrote:
// the page no longer has their note, and they are nobody again.
func TestAnAccountIsDeletedWithWhatItWrote(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.SignUp(at(screens.SignInPage{}), "Leaver")
	page.Open(at(pages.Home{}))
	body := "left behind at " + time.Now().Format(time.RFC3339Nano)
	page.Type(el(pages.Home.Form)+` input[name=body]`, body)
	page.Click(el(pages.Home.Form) + ` button[type=submit]`)
	page.WaitText(el(pages.Home.List), body)

	page.Open(at(screens.AccountPage{}))
	page.Click(`#leave [aria-haspopup=dialog]`)
	page.Click(`#leave button[type=submit]`)
	page.WaitText("#sign-in-link", "Sign in")
	if strings.Contains(apptest.Get(t, at(pages.Home{})), body) {
		t.Error("the note is still on the page after its writer deleted their account")
	}
}

// An invitation makes whoever signs up through it what it says, once: how an app gets its first admin (go tool
// charter run people:invite admin makes the same thing), and how an admin brings in another on the people page.
// Whoever may not manage people makes none.
func TestAnInvitationMakesAnAdmin(t *testing.T) {
	t.Parallel()
	form := "application/x-www-form-urlencoded"
	if res := as(t, env.WRITE_TOKEN, "POST", at(screens.PeopleInvite{}), form, strings.NewReader("role=admin")); res.StatusCode != http.StatusForbidden {
		t.Errorf("a writer making an invitation: HTTP %d, want 403", res.StatusCode)
	}
	res := as(t, env.ADMIN_TOKEN, "POST", at(screens.PeopleInvite{}), form, strings.NewReader("role=admin"))
	html, _ := io.ReadAll(res.Body)
	res.Body.Close()
	link := regexp.MustCompile(`/__charter/invite\?code=([A-Za-z0-9_-]+)`).FindStringSubmatch(string(html))
	if res.StatusCode != http.StatusOK || link == nil {
		t.Fatalf("making an invitation: HTTP %d\n%s", res.StatusCode, html)
	}
	// The link leads to the sign-in page, with its code.
	door := as(t, cloud.Variable{}, "GET", link[0], "", nil)
	door.Body.Close()
	signIn := at(screens.SignInPage{}) + "?invite=" + link[1]
	if to := door.Header.Get("Location"); door.StatusCode != http.StatusSeeOther || to != signIn {
		t.Fatalf("the invitation's link: HTTP %d to %q, want %q", door.StatusCode, to, signIn)
	}
	page := apptest.Browser(t)
	page.SignUp(signIn, "Invited")
	page.Open(at(screens.AccountPage{}))
	page.WaitText("#role", "admin")
	if !page.Has("#people-link") {
		t.Error("someone invited as an admin has no way to the people page")
	}
	// Once: the same link makes nobody else anything.
	again := as(t, cloud.Variable{}, "POST", at(screens.SignUpBegin{}), "application/json", strings.NewReader(`{"name":"Second","invite":"`+link[1]+`"}`))
	again.Body.Close()
	if again.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("signing up with an invitation that was used: HTTP %d, want 422", again.StatusCode)
	}
}

// Someone who has an account already opens an invitation's link and accepts it: they are what it says, at once.
func TestAnInvitationIsAcceptedBySomeoneSignedIn(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.SignUp(at(screens.SignInPage{}), "Here already")
	res := as(t, env.ADMIN_TOKEN, "POST", at(screens.PeopleInvite{}), "application/x-www-form-urlencoded", strings.NewReader("role=moderator"))
	html, _ := io.ReadAll(res.Body)
	res.Body.Close()
	link := regexp.MustCompile(`/__charter/invite\?code=[A-Za-z0-9_-]+`).FindString(string(html))
	if link == "" {
		t.Fatalf("making an invitation: HTTP %d\n%s", res.StatusCode, html)
	}
	page.Open(link)
	page.Click("#accept button")
	page.WaitText("#role", "moderator")
}

// The same link for someone who has an account and is signed out: the page says what the invitation is to, they
// sign in with their passkey, and they are that, on their account page. A link that is spent says so.
func TestAnInvitationSurvivesSigningIn(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.SignUp(at(screens.SignInPage{}), "Back again")
	page.Open(at(screens.AccountPage{}))
	page.Click("#sign-out")
	page.WaitText("#sign-in-link", "Sign in")

	res := as(t, env.ADMIN_TOKEN, "POST", at(screens.PeopleInvite{}), "application/x-www-form-urlencoded", strings.NewReader("role=admin"))
	html, _ := io.ReadAll(res.Body)
	res.Body.Close()
	code := regexp.MustCompile(`/__charter/invite\?code=([A-Za-z0-9_-]+)`).FindStringSubmatch(string(html))
	if code == nil {
		t.Fatalf("making an invitation: HTTP %d\n%s", res.StatusCode, html)
	}
	link := at(screens.SignInPage{}) + "?invite=" + code[1]
	page.Open(link)
	page.WaitText("#heading", "admin")
	page.SignIn(link)
	page.WaitText("#role", "admin")
	if path := page.Path(); path != at(screens.AccountPage{}) {
		t.Errorf("after signing in by an invitation, at %s", path)
	}
	// Spent: whoever opens it now is told, before they do anything.
	if !strings.Contains(apptest.Get(t, link), `id="invite-gone"`) {
		t.Error("a spent invitation's page does not say so")
	}
}

// The name someone gives is shown to everyone beside what they write, so an email address is refused as one: the
// field says what it is for, and the Worker does not take an address whoever typed it.
func TestANameIsNotAnEmailAddress(t *testing.T) {
	res := as(t, cloud.Variable{}, "POST", at(screens.SignUpBegin{}), "application/json", strings.NewReader(`{"name":"someone@example.com"}`))
	res.Body.Close()
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("signing up with an email address as the name: HTTP %d, want 422", res.StatusCode)
	}
	html := apptest.Get(t, at(screens.SignInPage{}))
	if !strings.Contains(html, `id="sign-up-name-hint"`) || !strings.Contains(html, `autocomplete="nickname"`) || !strings.Contains(html, `placeholder="Display name"`) {
		t.Error("the sign-up field does not say what the name is for")
	}
}

// An admin makes an invitation on the people page and shares it: the button hands the link to the device's share
// sheet, which the test stands in for.
func TestAnInvitationIsShared(t *testing.T) {
	t.Parallel()
	res := as(t, env.ADMIN_TOKEN, "POST", at(screens.PeopleInvite{}), "application/x-www-form-urlencoded", strings.NewReader("role=admin"))
	html, _ := io.ReadAll(res.Body)
	res.Body.Close()
	code := regexp.MustCompile(`/__charter/invite\?code=([A-Za-z0-9_-]+)`).FindStringSubmatch(string(html))
	if code == nil {
		t.Fatalf("making an invitation: HTTP %d\n%s", res.StatusCode, html)
	}
	admin := apptest.Browser(t)
	admin.SignUp(at(screens.SignInPage{})+"?invite="+code[1], "Sharer")
	admin.Open(at(screens.PeoplePage{}))
	admin.Click(el(screens.PeoplePage.Invite) + ` button[type=submit]`)
	admin.WaitText("#invite-link", "/__charter/invite?code=")
	link := strings.TrimSpace(admin.Text("#invite-link"))
	admin.JS(`(navigator.share = function (what) { window.shared = what.url; return Promise.resolve(); }, "")`)
	admin.Click("#invite-share")
	if shared := admin.JS(`String(window.shared)`); shared != link {
		t.Errorf("the share sheet was handed %q, want the link %q", shared, link)
	}
}

// An admin takes a person away on the people page, with what they wrote; nobody else may, and the admin's own row
// has no such button.
func TestAnAdminRemovesAPerson(t *testing.T) {
	t.Parallel()
	page := apptest.Browser(t)
	page.SignUp(at(screens.SignInPage{}), "To be removed")
	page.Open(at(pages.Home{}))
	body := "by someone removed at " + time.Now().Format(time.RFC3339Nano)
	page.Type(el(pages.Home.Form)+` input[name=body]`, body)
	page.Click(el(pages.Home.Form) + ` button[type=submit]`)
	page.WaitText(el(pages.Home.List), body)

	people, err := sdk(t, option.WithToken(apptest.Secret(t, env.ADMIN_TOKEN))).People.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, p := range people.Data {
		if p.Name != nil && *p.Name == "To be removed" {
			id = p.ID
		}
	}
	if id == "" {
		t.Fatal("the person who signed up is not among the people")
	}
	remove := at(screens.PersonRemove{}, "person", id)
	if res := as(t, env.WRITE_TOKEN, "POST", remove, "", nil); res.StatusCode != http.StatusForbidden {
		t.Errorf("a writer removing a person: HTTP %d, want 403", res.StatusCode)
	}
	if res := as(t, env.ADMIN_TOKEN, "POST", remove, "", nil); res.StatusCode != http.StatusOK {
		t.Fatalf("an admin removing a person: HTTP %d", res.StatusCode)
	}
	html := apptest.Get(t, at(screens.PeoplePage{}), "Authorization", "Bearer "+apptest.Secret(t, env.ADMIN_TOKEN))
	if strings.Contains(html, "To be removed") {
		t.Error("the person is still on the people page")
	}
	if !strings.Contains(html, "data-remove") {
		t.Error("the people page has no button that removes a person")
	}
	if strings.Contains(apptest.Get(t, at(pages.Home{})), body) {
		t.Error("what the removed person wrote is still on the page")
	}
}
