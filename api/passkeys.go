package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/joeblew999/charter/app"
)

// How one of the app's People signs in: with a passkey (WebAuthn, https://www.w3.org/TR/webauthn-3/). The device
// holds the key and asks its owner (a finger, a face, a PIN); the app keeps the public half and never a password.
// The ceremony is github.com/go-webauthn/webauthn's, run in the Worker; these are its four steps as the sign-in
// screen's routes call them, and what follows from them: a session, signing out, and the tokens someone makes for
// their programs.
//
// A passkey belongs to a host name, so a page is signed in on at the address it is served under: localhost under
// charter dev (an IP address has no passkeys), the Worker's own name deployed.

// ceremonyCookie names the sign-in that was started, between its two steps.
const ceremonyCookie = "ceremony"

// passkeyUser is a Person as the ceremony reads one.
type passkeyUser struct {
	Person
	credentials []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte                         { return []byte(u.ID) }
func (u passkeyUser) WebAuthnName() string                       { return u.Name }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.Name }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// started is what a ceremony keeps between its steps.
type started struct {
	Person  Person
	Session webauthn.SessionData
	// Invite is the invitation the account is made with, as it is kept: taken for good when the account is.
	Invite string `json:",omitempty"`
}

// An invitation is a link that makes whoever signs up through it what it says: how an app gets its first admin, and
// how one person brings in another as more than a newcomer. It works once, for an hour. It is kept as a started
// sign-in is, by the SHA-256 of its code, so what is kept invites no one.

// InviteLasts is how long an invitation can be taken up.
const InviteLasts = time.Hour

// InviteID is where an invitation with that code is kept (Accounts' Begin and Take; the ceremonies table).
func InviteID(code string) string { return invitePrefix + hashOf(code) }

// invitePrefix begins what an invitation is kept under, among the sign-ins that were started.
const invitePrefix = "invite:"

// Invite makes an invitation to be role, for whoever may manage people, and gives its code: app.InvitePath with
// ?code= is its link. charter run people:invite makes one from the command line.
func (a *Auth) Invite(r *http.Request, role string) (string, error) {
	accounts, err := a.managing(r)
	if err != nil {
		return "", err
	}
	if _, ok := a.people.Roles[role]; !ok {
		return "", app.Status(http.StatusUnprocessableEntity, "no such role: "+role)
	}
	code, _ := newToken()
	return code, accounts.Begin(r.Context(), InviteID(code), role, time.Now().Add(InviteLasts))
}

// Accept gives the person signed in what an invitation says: for someone who has an account already. The
// invitation is taken for good.
func (a *Auth) Accept(r *http.Request, code string) error {
	who, accounts, err := a.signedIn(r)
	if err != nil {
		return err
	}
	role, err := accounts.Take(r.Context(), InviteID(code))
	if err != nil {
		return app.Status(http.StatusUnprocessableEntity, "this invitation was used already, or is more than an hour old: ask for a new one")
	}
	if _, ok := a.people.Roles[role]; !ok {
		return app.Status(http.StatusUnprocessableEntity, "the invitation is to be "+role+", which the app has no such role for")
	}
	return accounts.SetRole(r.Context(), who.Subject, role)
}

// Invited is the role an invitation with that code gives, ok false when there is none any more: what a page says
// to whoever opened its link.
func (a *Auth) Invited(r *http.Request, code string) (role string, ok bool) {
	if a.people == nil || code == "" {
		return "", false
	}
	accounts, err := a.people.Accounts()
	if err != nil {
		return "", false
	}
	role, err = a.invited(r.Context(), accounts, code)
	return role, err == nil
}

// invited is the role an invitation gives. It stays where it is until the account is made, or it is accepted:
// one invitation, one person.
func (a *Auth) invited(ctx context.Context, accounts Accounts, code string) (role string, err error) {
	role, err = accounts.Look(ctx, InviteID(code))
	if err != nil {
		return "", app.Status(http.StatusUnprocessableEntity, "this invitation was used already, or is more than an hour old: ask for a new one")
	}
	if _, ok := a.people.Roles[role]; !ok {
		return "", app.Status(http.StatusUnprocessableEntity, "the invitation is to be "+role+", which the app has no such role for")
	}
	return role, nil
}

func (a *Auth) ceremony(r *http.Request) (*webauthn.WebAuthn, Accounts, error) {
	if a.people == nil {
		return nil, nil, errors.New("api: the app's Auth has no People to sign in")
	}
	origin := Origin(r)
	at, err := url.Parse(origin)
	if err != nil {
		return nil, nil, err
	}
	w, err := webauthn.New(&webauthn.Config{RPID: at.Hostname(), RPDisplayName: a.people.Name, RPOrigins: []string{origin}})
	if err != nil {
		return nil, nil, err
	}
	accounts, err := a.people.Accounts()
	return w, accounts, err
}

func secure(r *http.Request) bool { return strings.HasPrefix(Origin(r), "https://") }

// begin keeps what was started and tells the browser what to ask the device.
func (a *Auth) begin(w http.ResponseWriter, r *http.Request, accounts Accounts, s started, options any) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	id := randomID()
	if err := accounts.Begin(r.Context(), id, string(data), time.Now().Add(ceremonyLasts)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: ceremonyCookie, Value: id, Path: "/", MaxAge: int(ceremonyLasts.Seconds()), HttpOnly: true, Secure: secure(r), SameSite: http.SameSiteStrictMode})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	return json.NewEncoder(w).Encode(options)
}

// take is what was started, once.
func (a *Auth) take(w http.ResponseWriter, r *http.Request, accounts Accounts) (started, error) {
	var s started
	c, err := r.Cookie(ceremonyCookie)
	if err != nil {
		return s, app.Status(http.StatusBadRequest, "no sign-in was started here, or it took too long: start again")
	}
	http.SetCookie(w, &http.Cookie{Name: ceremonyCookie, Path: "/", MaxAge: -1})
	data, err := accounts.Take(r.Context(), c.Value)
	if err != nil {
		return s, app.Status(http.StatusBadRequest, "no sign-in was started here, or it took too long: start again")
	}
	return s, json.Unmarshal([]byte(data), &s)
}

// refused is a ceremony the device or the browser did not complete as asked: the caller's doing, said plainly.
func refused(err error) error {
	var detail *protocol.Error
	if errors.As(err, &detail) {
		return app.Status(http.StatusBadRequest, "the passkey was not accepted: "+detail.Details)
	}
	return app.Status(http.StatusBadRequest, "the passkey was not accepted: "+err.Error())
}

// SignUpBegin starts making an account: the request's JSON has the name (at most 60 characters) someone gives
// themselves. It answers what the browser passes to navigator.credentials.create.
func (a *Auth) SignUpBegin(w http.ResponseWriter, r *http.Request) error {
	wa, accounts, err := a.ceremony(r)
	if err != nil {
		return err
	}
	var in struct{ Name, Invite string }
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&in)
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 60 {
		return app.Status(http.StatusUnprocessableEntity, "a name is needed, of at most 60 characters")
	}
	// The name is shown to everyone, beside what its person writes: an address typed into it would be published.
	if strings.Contains(in.Name, "@") {
		return app.Status(http.StatusUnprocessableEntity, "the name is shown to everyone: choose one that is not an email address")
	}
	who := Person{ID: randomID(), Name: in.Name, Role: a.people.Role}
	if in.Invite != "" {
		if who.Role, err = a.invited(r.Context(), accounts, in.Invite); err != nil {
			return err
		}
	}
	// A passkey the device itself lists (a "discoverable" one), so signing in asks for no name.
	options, session, err := wa.BeginRegistration(passkeyUser{Person: who},
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{ResidentKey: protocol.ResidentKeyRequirementRequired, UserVerification: protocol.VerificationPreferred}))
	if err != nil {
		return err
	}
	return a.begin(w, r, accounts, started{Person: who, Session: *session, Invite: in.Invite}, options)
}

// SignUpFinish takes what the device made: the account and its passkey are kept, and the caller is signed in.
func (a *Auth) SignUpFinish(w http.ResponseWriter, r *http.Request) error {
	wa, accounts, err := a.ceremony(r)
	if err != nil {
		return err
	}
	s, err := a.take(w, r, accounts)
	if err != nil {
		return err
	}
	credential, err := wa.FinishRegistration(passkeyUser{Person: s.Person}, s.Session, r)
	if err != nil {
		return refused(err)
	}
	// The invitation goes as the account comes: someone else who started with the same link is too late.
	if s.Invite != "" {
		if _, err := accounts.Take(r.Context(), InviteID(s.Invite)); err != nil {
			return app.Status(http.StatusUnprocessableEntity, "this invitation was used already: ask for a new one")
		}
	}
	if err := accounts.Create(r.Context(), s.Person); err != nil {
		return err
	}
	if err := keep(r, accounts, s.Person.ID, credential); err != nil {
		return err
	}
	return a.signIn(w, r, accounts, s.Person)
}

// SignInBegin starts signing in. It answers what the browser passes to navigator.credentials.get: the device
// offers the passkeys it has for this app.
func (a *Auth) SignInBegin(w http.ResponseWriter, r *http.Request) error {
	wa, accounts, err := a.ceremony(r)
	if err != nil {
		return err
	}
	options, session, err := wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		return err
	}
	return a.begin(w, r, accounts, started{Session: *session}, options)
}

// SignInFinish takes what the device signed: whoever the passkey belongs to is signed in.
func (a *Auth) SignInFinish(w http.ResponseWriter, r *http.Request) error {
	wa, accounts, err := a.ceremony(r)
	if err != nil {
		return err
	}
	s, err := a.take(w, r, accounts)
	if err != nil {
		return err
	}
	var who Person
	find := func(_, userHandle []byte) (webauthn.User, error) {
		who, err = accounts.Person(r.Context(), string(userHandle))
		if err != nil {
			return nil, err
		}
		kept, err := accounts.Passkeys(r.Context(), who.ID)
		if err != nil {
			return nil, err
		}
		u := passkeyUser{Person: who}
		for _, k := range kept {
			var c webauthn.Credential
			if json.Unmarshal([]byte(k), &c) == nil {
				u.credentials = append(u.credentials, c)
			}
		}
		return u, nil
	}
	credential, err := wa.FinishDiscoverableLogin(find, s.Session, r)
	if err != nil {
		return refused(err)
	}
	// How often it was used is kept: a passkey that was copied shows by counting backwards.
	if err := keep(r, accounts, who.ID, credential); err != nil {
		return err
	}
	return a.signIn(w, r, accounts, who)
}

func keep(r *http.Request, accounts Accounts, userID string, credential *webauthn.Credential) error {
	kept, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	return accounts.AddPasskey(r.Context(), userID, protocol.URLEncodedBase64(credential.ID).String(), string(kept))
}

// signIn issues a session for who and gives it to the browser.
func (a *Auth) signIn(w http.ResponseWriter, r *http.Request, accounts Accounts, who Person) error {
	token, hash := newToken()
	expires := time.Now().Add(sessionLasts)
	// With no scopes of its own: a session may do what its person may, whatever that becomes.
	if err := accounts.Issue(r.Context(), who.ID, SessionScheme, "", hash, nil, expires); err != nil {
		return err
	}
	// Not readable by scripts, sent on a link from elsewhere but on no request another site makes for itself.
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: token, Path: "/", Expires: expires, HttpOnly: true, Secure: secure(r), SameSite: http.SameSiteLaxMode})
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	return json.NewEncoder(w).Encode(map[string]string{"name": who.Name})
}

// SignOut ends the request's session: its token is gone from the database, and its cookie from the browser.
func (a *Auth) SignOut(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(SessionCookie); err == nil && a.people != nil {
		who, _ := a.Caller(r)
		accounts, err := a.people.Accounts()
		if err != nil {
			return err
		}
		if err := accounts.Revoke(r.Context(), who.Subject, 0, hashOf(c.Value)); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Path: "/", MaxAge: -1})
	return nil
}

// Leave deletes the account of the person signed in: what the app keeps of theirs (People's Leaving), then the
// person, their passkeys, sessions and tokens. It is theirs to do and no one else's: a program with one of their
// tokens may not, only they on a page.
func (a *Auth) Leave(w http.ResponseWriter, r *http.Request) error {
	who, accounts, err := a.signedIn(r)
	if err != nil {
		return err
	}
	person := Person{ID: who.Subject, Name: who.Person, Role: who.Role}
	if a.people.Leaving != nil {
		if err := a.people.Leaving(r.Context(), person); err != nil {
			return err
		}
	}
	if err := accounts.Remove(r.Context(), person.ID); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Path: "/", MaxAge: -1})
	return nil
}

// NewToken makes a token for a program of the person signed in, with a name to know it by and the scopes it may
// use: theirs, or fewer. The token is returned once and never kept; a program sends it as Authorization: Bearer.
func (a *Auth) NewToken(r *http.Request, name string, scopes ...string) (string, error) {
	who, accounts, err := a.signedIn(r)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 60 {
		return "", app.Status(http.StatusUnprocessableEntity, "a token needs a name, of at most 60 characters")
	}
	// With no scopes of its own, a token may do what its maker may, whatever that becomes.
	if !who.May(scopesOfWords(scopes)...) {
		return "", app.Status(http.StatusForbidden, "a token may do what you may, or less")
	}
	token, hash := newToken()
	return token, accounts.Issue(r.Context(), who.Subject, TokenScheme, name, hash, scopes, time.Time{})
}

// Tokens are the tokens the person signed in made for their programs.
func (a *Auth) Tokens(r *http.Request) ([]IssuedToken, error) {
	who, accounts, err := a.signedIn(r)
	if err != nil {
		return nil, err
	}
	return accounts.Tokens(r.Context(), who.Subject)
}

// RevokeToken takes one of them away.
func (a *Auth) RevokeToken(r *http.Request, id int64) error {
	who, accounts, err := a.signedIn(r)
	if err != nil {
		return err
	}
	return accounts.Revoke(r.Context(), who.Subject, id, "")
}

// signedIn is the person the request's session is of: only someone who signed in themselves manages tokens, not a
// program holding one.
func (a *Auth) signedIn(r *http.Request) (Caller, Accounts, error) {
	who, _ := a.Caller(r)
	if a.people == nil || who.Scheme != SessionScheme {
		return who, nil, app.Status(http.StatusUnauthorized, "sign in to do this")
	}
	accounts, err := a.people.Accounts()
	return who, accounts, err
}
