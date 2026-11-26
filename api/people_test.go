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
	next int64
}

func newFakeAccounts() *fakeAccounts {
	return &fakeAccounts{people: map[string]Person{}, tokens: map[string]struct {
		user, kind, name string
		scopes           []string
		id               int64
	}{}}
}

func (m *fakeAccounts) ByToken(_ context.Context, hash string) (Person, string, error) {
	t, ok := m.tokens[hash]
	if !ok {
		return Person{}, "", ErrNoAccount
	}
	p := m.people[t.user]
	p.Scopes = t.scopes
	return p, t.kind, nil
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
func (m *fakeAccounts) Begin(context.Context, string, string, time.Time) error { return nil }
func (m *fakeAccounts) Take(context.Context, string) (string, error)           { return "", ErrNoAccount }

// A person's session and the token they make for a program are the same caller: by the cookie, by Authorization:
// Bearer. A token may do less than the person, never more; revoked, it is nobody's; a session that has ended is
// nobody, with no error (a browser keeps cookies).
func TestPeopleAreOneCallerByCookieAndByToken(t *testing.T) {
	accounts := newFakeAccounts()
	ada := Person{ID: "u1", Name: "Ada", Scopes: []string{"read", "write"}}
	accounts.Create(t.Context(), ada)
	auth := NewAuth(nil, People{Name: "Notes", Accounts: func() (Accounts, error) { return accounts, nil }, Scopes: ada.Scopes})

	session, hash := newToken()
	accounts.Issue(t.Context(), ada.ID, SessionScheme, "", hash, ada.Scopes, time.Time{})
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

	gone := httptest.NewRequest("GET", "/", nil)
	gone.AddCookie(&http.Cookie{Name: SessionCookie, Value: tokenPrefix + "ended"})
	if c, err := auth.Caller(gone); err != nil || c.SignedIn() {
		t.Errorf("a session that has ended: %+v %v", c, err)
	}
	if err := auth.Allow(gone); err == nil {
		t.Error("nobody passed a check for someone signed in")
	}
}
