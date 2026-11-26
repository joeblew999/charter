package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeAccounts is Accounts in fakeAccounts: what a test (or an app's own tests) gives People.
type fakeAccounts struct {
	people map[string]Person
	tokens map[string]struct {
		user, kind, name string
		scopes           []string
		id               int64
	}
	next    int64
	started map[string]string
}

func newFakeAccounts() *fakeAccounts {
	return &fakeAccounts{people: map[string]Person{}, tokens: map[string]struct {
		user, kind, name string
		scopes           []string
		id               int64
	}{}}
}

func (m *fakeAccounts) ByToken(_ context.Context, hash string) (Person, string, []string, error) {
	t, ok := m.tokens[hash]
	if !ok {
		return Person{}, "", nil, ErrNoAccount
	}
	return m.people[t.user], t.kind, t.scopes, nil
}

func (m *fakeAccounts) Everyone(context.Context) (out []Person, err error) {
	for _, p := range m.people {
		out = append(out, p)
	}
	return out, nil
}

func (m *fakeAccounts) SetRole(_ context.Context, id, role string) error {
	p, ok := m.people[id]
	if !ok {
		return ErrNoAccount
	}
	p.Role = role
	m.people[id] = p
	return nil
}
func (m *fakeAccounts) Person(_ context.Context, id string) (Person, error) {
	p, ok := m.people[id]
	if !ok {
		return p, ErrNoAccount
	}
	return p, nil
}
func (m *fakeAccounts) Create(_ context.Context, p Person) error                 { m.people[p.ID] = p; return nil }
func (m *fakeAccounts) Passkeys(context.Context, string) ([]string, error)       { return nil, nil }
func (m *fakeAccounts) AddPasskey(context.Context, string, string, string) error { return nil }
func (m *fakeAccounts) Issue(_ context.Context, userID, kind, name, hash string, scopes []string, _ time.Time) error {
	m.next++
	m.tokens[hash] = struct {
		user, kind, name string
		scopes           []string
		id               int64
	}{userID, kind, name, scopes, m.next}
	return nil
}
func (m *fakeAccounts) Tokens(_ context.Context, userID string) (out []IssuedToken, err error) {
	for _, t := range m.tokens {
		if t.user == userID && t.kind == TokenScheme {
			out = append(out, IssuedToken{ID: t.id, Name: t.name, Scopes: t.scopes})
		}
	}
	return out, nil
}
func (m *fakeAccounts) Revoke(_ context.Context, userID string, id int64, hash string) error {
	for h, t := range m.tokens {
		if t.user == userID && (t.id == id || h == hash) {
			delete(m.tokens, h)
		}
	}
	return nil
}
func (m *fakeAccounts) Remove(_ context.Context, id string) error {
	delete(m.people, id)
	for h, t := range m.tokens {
		if t.user == id {
			delete(m.tokens, h)
		}
	}
	return nil
}
func (m *fakeAccounts) Begin(_ context.Context, id, data string, _ time.Time) error {
	if m.started == nil {
		m.started = map[string]string{}
	}
	m.started[id] = data
	return nil
}
func (m *fakeAccounts) Look(_ context.Context, id string) (string, error) {
	data, ok := m.started[id]
	if !ok {
		return "", ErrNoAccount
	}
	return data, nil
}
func (m *fakeAccounts) Take(_ context.Context, id string) (string, error) {
	data, ok := m.started[id]
	if !ok {
		return "", ErrNoAccount
	}
	delete(m.started, id)
	return data, nil
}

// A person's session and the token they make for a program are the same caller: by the cookie, by Authorization:
// Bearer. A token may do less than the person, never more; revoked, it is nobody's; a session that has ended is
// nobody, with no error (a browser keeps cookies).
func TestPeopleAreOneCallerByCookieAndByToken(t *testing.T) {
	accounts := newFakeAccounts()
	ada := Person{ID: "u1", Name: "Ada", Role: "member"}
	accounts.Create(t.Context(), ada)
	roles := map[string][]string{"reader": {"read"}, "member": {"read", "write"}, "admin": {"read", "write", "admin"}}
	auth := NewAuth(nil, People{Name: "Notes", Accounts: func() (Accounts, error) { return accounts, nil }, Roles: roles, Role: "member"},
		Token{Secret: "ADMIN_TOKEN", Scopes: []string{"admin"}})

	session, hash := newToken()
	accounts.Issue(t.Context(), ada.ID, SessionScheme, "", hash, nil, time.Time{})
	page := httptest.NewRequest("GET", "/", nil)
	page.AddCookie(&http.Cookie{Name: SessionCookie, Value: session})
	if c, _ := auth.Caller(page); c.Scheme != SessionScheme || c.Person != "Ada" || !c.May("write") {
		t.Fatalf("the session's caller: %+v", c)
	}
	if err := auth.Allow(page, "write"); err != nil {
		t.Errorf("someone signed in who may write: %v", err)
	}

	made, err := auth.NewToken(page, "a script", "read")
	if err != nil {
		t.Fatal(err)
	}
	program := httptest.NewRequest("GET", "/", nil)
	program.Header.Set("Authorization", "Bearer "+made)
	c, _ := auth.Caller(program)
	if c.Scheme != TokenScheme || c.Subject != "u1" || !c.May("read") || c.May("write") {
		t.Errorf("the token's caller: %+v", c)
	}
	if err := auth.Allow(program, "write"); err == nil {
		t.Error("a token made to read may write")
	}
	if _, err := auth.NewToken(page, "too much", "admin"); err == nil {
		t.Error("a token was made that may do what its maker may not")
	}
	if _, err := auth.NewToken(program, "by a program"); err == nil {
		t.Error("a program made a token: only someone signed in themselves does")
	}

	tokens, _ := auth.Tokens(page)
	if len(tokens) != 1 {
		t.Fatalf("tokens: %+v", tokens)
	}
	auth.RevokeToken(page, tokens[0].ID)
	if c, _ := auth.Caller(program); c.SignedIn() {
		t.Errorf("a revoked token is still %+v", c)
	}

	// What someone is decides what they may do, at once: in the session they have open.
	if err := auth.SetRole(page, ada.ID, "admin"); err == nil {
		t.Error("a member changed a role")
	}
	accounts.SetRole(t.Context(), ada.ID, "reader")
	if err := auth.Allow(httptestWith(session), "write"); err == nil {
		t.Error("someone made a reader still writes, in the session they had")
	}
	accounts.SetRole(t.Context(), ada.ID, "admin")
	asAdmin := httptestWith(session)
	if people, err := auth.Everyone(asAdmin); err != nil || len(people) != 1 {
		t.Errorf("an admin's list of people: %v %v", people, err)
	}
	if err := auth.SetRole(asAdmin, ada.ID, "no-such-role"); err == nil {
		t.Error("a role that is none was given")
	}

	gone := httptest.NewRequest("GET", "/", nil)
	gone.AddCookie(&http.Cookie{Name: SessionCookie, Value: tokenPrefix + "ended"})
	if c, err := auth.Caller(gone); err != nil || c.SignedIn() {
		t.Errorf("a session that has ended: %+v %v", c, err)
	}
	if err := auth.Allow(gone); err == nil {
		t.Error("nobody passed a check for someone signed in")
	}
}

// httptestWith is a page's request with a session's cookie.
func httptestWith(session string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: session})
	return r
}

// An account is deleted by its person, signed in: what the app keeps of theirs goes first (Leaving), then they do,
// with their session and the tokens they made. A program with one of their tokens may not.
func TestLeavingTakesTheAccountAndWhatIsTheirs(t *testing.T) {
	accounts := newFakeAccounts()
	ada := Person{ID: "u1", Name: "Ada", Role: "member"}
	accounts.Create(t.Context(), ada)
	var left []string
	auth := NewAuth(nil, People{Name: "Notes", Accounts: func() (Accounts, error) { return accounts, nil },
		Roles: map[string][]string{"member": {"read", "write"}}, Role: "member",
		Leaving: func(_ context.Context, who Person) error { left = append(left, who.ID); return nil }})
	session, hash := newToken()
	accounts.Issue(t.Context(), ada.ID, SessionScheme, "", hash, nil, time.Time{})
	made, err := auth.NewToken(httptestWith(session), "a script")
	if err != nil {
		t.Fatal(err)
	}
	program := httptest.NewRequest("POST", "/", nil)
	program.Header.Set("Authorization", "Bearer "+made)
	if err := auth.Leave(httptest.NewRecorder(), program); err == nil || len(left) != 0 {
		t.Fatalf("a program deleted its person's account: %v, %v", err, left)
	}
	w := httptest.NewRecorder()
	if err := auth.Leave(w, httptestWith(session)); err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0] != ada.ID {
		t.Errorf("the app was told of %v leaving, want %s", left, ada.ID)
	}
	if c, _ := auth.Caller(httptestWith(session)); c.SignedIn() {
		t.Errorf("the session of a deleted account is still %+v", c)
	}
	if c, _ := auth.Caller(program); c.SignedIn() {
		t.Errorf("the token of a deleted account is still %+v", c)
	}
	if cookie := w.Result().Cookies(); len(cookie) != 1 || cookie[0].MaxAge >= 0 {
		t.Errorf("the session's cookie was not taken back: %v", cookie)
	}
}

// An invitation is made by whoever may manage people, to a role the app has; it gives that role once.
func TestAnInvitationGivesItsRoleOnce(t *testing.T) {
	accounts := newFakeAccounts()
	auth := NewAuth(func(name string) string { return map[string]string{"ADMIN_TOKEN": "operator"}[name] },
		People{Name: "Notes", Accounts: func() (Accounts, error) { return accounts, nil },
			Roles: map[string][]string{"member": {"read"}, "admin": {"read", "admin"}}, Role: "member"},
		Token{Secret: "ADMIN_TOKEN", Scopes: []string{"admin"}})
	operator := httptest.NewRequest("POST", "/", nil)
	operator.Header.Set("Authorization", "Bearer operator")
	if _, err := auth.Invite(httptest.NewRequest("POST", "/", nil), "admin"); err == nil {
		t.Error("nobody made an invitation")
	}
	if _, err := auth.Invite(operator, "owner"); err == nil {
		t.Error("an invitation was made to a role the app does not have")
	}
	code, err := auth.Invite(operator, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, kept := accounts.started[code]; kept {
		t.Error("the invitation is kept by its code, which whoever reads the table could use")
	}
	if role, err := auth.invited(t.Context(), accounts, code); err != nil || role != "admin" {
		t.Fatalf("the invitation gives %q, %v", role, err)
	}
	// Looked at, it is still there: a page says what it is before anyone takes it up.
	if role, ok := auth.Invited(operator, code); !ok || role != "admin" {
		t.Fatalf("the invitation says %q, %v", role, ok)
	}
	// Taken for good, as the account is made.
	accounts.Take(t.Context(), InviteID(code))
	if _, err := auth.invited(t.Context(), accounts, code); err == nil {
		t.Error("an invitation worked twice")
	}
	if _, err := auth.invited(t.Context(), accounts, "ch_made_up"); err == nil {
		t.Error("a code nobody made is an invitation")
	}
}
