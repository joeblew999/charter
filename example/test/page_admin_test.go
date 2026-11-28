package test

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/env"
	"github.com/joeblew999/charter/example/pages"
	"github.com/joeblew999/charter/example/pages/screens"
)

// The admin screen (charter's): what an admin sees of the system. Its pages are for whoever may manage people and
// for no one else, by the same rule as every other page; each shows what the Worker really has.

// admin is a page of the admin screen as the operator sees it.
func admin(t *testing.T, path string) string {
	t.Helper()
	return apptest.Get(t, path, "Authorization", "Bearer "+apptest.Secret(t, env.ADMIN_TOKEN))
}

// Nobody is sent to sign in; someone who may write and not manage is refused; the header offers the way in only to
// whoever may take it.
func TestTheAdminPagesAreForAdmins(t *testing.T) {
	for _, path := range []string{at(screens.AdminOverview{}), at(screens.AdminSystem{}), at(screens.AdminDatabase{})} {
		if res := as(t, cloud.Variable{}, "GET", path, "", nil); res.StatusCode != http.StatusSeeOther || !strings.Contains(res.Header.Get("Location"), "sign-in") {
			t.Errorf("nobody opening %s: HTTP %d to %q, want a redirect to sign in", path, res.StatusCode, res.Header.Get("Location"))
		}
		if res := as(t, env.WRITE_TOKEN, "GET", path, "", nil); res.StatusCode != http.StatusForbidden {
			t.Errorf("a writer opening %s: HTTP %d, want 403", path, res.StatusCode)
		}
	}
	if strings.Contains(apptest.Get(t, at(pages.Home{})), `id="admin-link"`) {
		t.Error("the header offers nobody the admin pages")
	}
	if !strings.Contains(admin(t, at(pages.Home{})), `id="admin-link"`) {
		t.Error("the header does not offer an admin the admin pages")
	}
}

// The overview says what the Worker is built with, how many people there are of each role, the app's own facts,
// and the invitations that wait: one made a moment ago is there, and gone once it is cancelled.
func TestTheAdminOverview(t *testing.T) {
	html := admin(t, at(screens.AdminOverview{}))
	for _, want := range []string{`id="admin-build"`, "charter", `data-role="admin"`, `data-role="member"`, `id="admin-facts"`, "People who wrote one"} {
		if !strings.Contains(html, want) {
			t.Errorf("the overview lacks %q", want)
		}
	}
	res := as(t, env.ADMIN_TOKEN, "POST", at(screens.PeopleInvite{}), "application/x-www-form-urlencoded", strings.NewReader("role=moderator"))
	made, _ := io.ReadAll(res.Body)
	res.Body.Close()
	code := regexp.MustCompile(`/__charter/invite\?code=([A-Za-z0-9_-]+)`).FindStringSubmatch(string(made))
	if code == nil {
		t.Fatalf("making an invitation: HTTP %d", res.StatusCode)
	}
	html = admin(t, at(screens.AdminOverview{}))
	cancel := regexp.MustCompile(`hx-post="([^"]*/invitations/[^"]+/cancel)"`).FindAllStringSubmatch(html, -1)
	if len(cancel) == 0 {
		t.Fatal("the overview does not list the invitation that was made")
	}
	if strings.Contains(html, code[1]) {
		t.Error("the overview shows an invitation's code, which is the invitation")
	}
	// The newest ends last: it is the last listed.
	if res := as(t, env.ADMIN_TOKEN, "POST", cancel[len(cancel)-1][1], "", nil); res.StatusCode != http.StatusOK {
		t.Fatalf("cancelling the invitation: HTTP %d", res.StatusCode)
	}
	if !strings.Contains(apptest.Get(t, at(screens.SignInPage{})+"?invite="+code[1]), `id="invite-gone"`) {
		t.Error("a cancelled invitation can still be taken up")
	}
}

// The system page lists what the Worker is bound to, by what each binding is, and its variables and secrets by
// name, never by value. A binding asked one small thing says it answers.
func TestTheAdminSystemPage(t *testing.T) {
	html := admin(t, at(screens.AdminSystem{}))
	for _, want := range []string{`id="admin-memory"`, ">DB<", ">database<", ">KV<", ">kv<", ">BUCKET<", ">bucket<", ">QUEUE<", ">queue<", ">ADMIN_TOKEN<", ">APP_NAME<"} {
		if !strings.Contains(html, want) {
			t.Errorf("the system page lacks %q", want)
		}
	}
	if secret := apptest.Secret(t, env.ADMIN_TOKEN); strings.Contains(html, secret) {
		t.Error("the system page shows a secret's value")
	}
	for _, binding := range []string{"DB", "KV", "BUCKET"} {
		res := as(t, env.ADMIN_TOKEN, "POST", at(screens.AdminProbe{}, "binding", binding), "application/x-www-form-urlencoded", strings.NewReader("kind=database"))
		row, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK || !strings.Contains(string(row), `data-probed="ok"`) {
			t.Errorf("asking %s: HTTP %d\n%s", binding, res.StatusCode, row)
		}
	}
	// A queue takes only messages: it is bound, and is not asked.
	res := as(t, env.ADMIN_TOKEN, "POST", at(screens.AdminProbe{}, "binding", "QUEUE"), "application/x-www-form-urlencoded", strings.NewReader("kind=queue"))
	row, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(row), `data-probed="bound"`) {
		t.Errorf("asking the queue: HTTP %d\n%s", res.StatusCode, row)
	}
	if res := as(t, env.ADMIN_TOKEN, "POST", at(screens.AdminProbe{}, "binding", "ADMIN_TOKEN"), "", nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("asking a secret as if it were a binding: HTTP %d, want 404", res.StatusCode)
	}
}

// The database page lists the tables with their rows, and the migrations that ran.
func TestTheAdminDatabasePage(t *testing.T) {
	html := admin(t, at(screens.AdminDatabase{}))
	for _, want := range []string{`data-table="notes"`, `data-table="users"`, `data-migrations="DB"`, "0001_"} {
		if !strings.Contains(html, want) {
			t.Errorf("the database page lacks %q", want)
		}
	}
	if strings.Contains(html, `data-table="d1_migrations"`) {
		t.Error("the database page lists wrangler's own table among the app's")
	}
}
