package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/cloud"
)

// People are the app's own users: who signed in on its pages (with a passkey: passkeys.go), and the tokens they
// made for their programs. It is one of the ways in an API trusts, beside Token, Access and OIDC, and the one a
// page's reader comes by, so a person on a page and a program with that person's token are the same Caller with
// the same scopes: what an operation of the contract needs (Needs("write")) is what a page needs (its Needs
// method), in the same words, checked by the same code (Auth).
//
// Someone who signed in has a session: a token the app issued, which their browser keeps as a cookie. A token
// they make for a program is the same kind of thing, sent as Authorization: Bearer. Neither is kept as it is: the
// database has its SHA-256.
type People struct {
	// Name is what the app is called where a passkey is kept (the browser's and the device's own lists).
	Name string
	// Accounts opens where users, passkeys and tokens are kept, for a request: D1Accounts("DB").
	Accounts func() (Accounts, error)
	// Roles are what people may be, each a name and the scopes it has: who may do what is said here once, and a
	// person has one role. Role is the one someone who signs up gets. Changing what a role may do changes it for
	// everyone who has it, at their next request.
	//
	//	Roles: map[string][]string{"member": {"read", "write"}, "moderator": {"read", "write", "moderate"}},
	//	Role:  "member",
	Roles map[string][]string
	Role  string
	// Manage is the scope that lets someone see everyone and change their roles (Auth.Everyone, Auth.SetRole).
	// Empty: "admin".
	Manage string
}

// scopes are what a role may do; a role that is no longer one of them is the one new people get.
func (p People) scopes(role string) []string {
	if s, ok := p.Roles[role]; ok {
		return s
	}
	return p.Roles[p.Role]
}

func (p People) manage() string {
	if p.Manage != "" {
		return p.Manage
	}
	return "admin"
}

func (People) trusted() {}

// SessionScheme is who vouched for a Caller that signed in on a page, and TokenScheme for a program with a token
// such a person made.
const (
	SessionScheme = "session"
	TokenScheme   = "token"
)

// SessionCookie is the cookie a session is kept in.
const SessionCookie = "session"

// tokenPrefix starts every token the app issues: what tells one from a secret of the Worker's before the database
// is asked.
const tokenPrefix = "ch_"

// How long a session lasts, and a sign-in that was started.
const (
	sessionLasts  = 30 * 24 * time.Hour
	ceremonyLasts = 5 * time.Minute
)

// ErrNoAccount is what Accounts answer for a user, a token or a sign-in that is not there (or no longer good).
var ErrNoAccount = errors.New("no such account")

// Person is one of the app's users.
type Person struct {
	ID   string // a random id: what a passkey is tied to
	Name string // what they called themselves
	Role string // what they are (People's Roles): what they may do follows from it
}

// IssuedToken is a token someone holds, as a page lists it: never the token itself, which is shown once, when it
// is made.
type IssuedToken struct {
	ID      int64
	Name    string
	Scopes  []string
	Created string
}

// Accounts is where People are kept. D1Accounts is the one a Worker uses; a test has one in memory.
type Accounts interface {
	// ByToken is who holds the token with this hash, its kind (SessionScheme or TokenScheme) and the scopes it was
	// made with (none: all its holder has): ErrNoAccount when none does, or it has expired.
	ByToken(ctx context.Context, hash string) (who Person, kind string, scopes []string, err error)
	// Person is the user with that id.
	Person(ctx context.Context, id string) (Person, error)
	// Create keeps a new user; Everyone is all of them, oldest first; SetRole changes what one is.
	Create(ctx context.Context, p Person) error
	Everyone(ctx context.Context) ([]Person, error)
	SetRole(ctx context.Context, id, role string) error
	// Passkeys are a user's passkeys, each as it was kept; AddPasskey keeps one, and again when it was used.
	Passkeys(ctx context.Context, userID string) ([]string, error)
	AddPasskey(ctx context.Context, userID, id, credential string) error
	// Issue keeps a token's hash for a user; Tokens are those a user made for programs; Revoke takes one away
	// (a session by its hash, a program's token by its id).
	Issue(ctx context.Context, userID, kind, name, hash string, scopes []string, expires time.Time) error
	Tokens(ctx context.Context, userID string) ([]IssuedToken, error)
	Revoke(ctx context.Context, userID string, id int64, hash string) error
	// Begin keeps what a sign-in that was started needs to be finished; Take gives it back, once.
	Begin(ctx context.Context, id, data string, expires time.Time) error
	Take(ctx context.Context, id string) (string, error)
}

// newToken is a token and its hash.
func newToken() (token, hash string) {
	b := make([]byte, 32)
	rand.Read(b)
	token = tokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return token, hashOf(token)
}

func hashOf(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// person is the Caller a token of the app's is: from Authorization: Bearer, or from the session cookie. A cookie
// that is no longer good is no caller (a browser keeps cookies); a bearer token that is not is refused.
func (g gate) person(ctx context.Context, bearerToken, cookies string) (Caller, bool, error) {
	token, fromCookie := bearerToken, false
	if token == "" {
		r := http.Request{Header: http.Header{"Cookie": {cookies}}}
		if c, err := r.Cookie(SessionCookie); err == nil {
			token, fromCookie = c.Value, true
		}
	}
	if !strings.HasPrefix(token, tokenPrefix) {
		return Caller{}, false, nil
	}
	accounts, err := g.people.Accounts()
	if err != nil {
		return Caller{}, false, err
	}
	who, kind, narrowed, err := accounts.ByToken(ctx, hashOf(token))
	switch {
	case errors.Is(err, ErrNoAccount) && fromCookie:
		return Caller{}, false, nil
	case errors.Is(err, ErrNoAccount):
		return Caller{}, false, errUnknownToken
	case err != nil:
		return Caller{}, false, err
	}
	// What they may do is their role's, now: a role that changed, or was taken away, has. A token made to do less
	// does that much of it.
	scopes := g.people.scopes(who.Role)
	if len(narrowed) > 0 {
		scopes = slices.DeleteFunc(slices.Clone(narrowed), func(s string) bool { return !slices.Contains(scopes, s) })
	}
	return Caller{Scheme: kind, Subject: who.ID, Person: who.Name, Role: who.Role, Scopes: scopes}, true, nil
}

// May reports whether the caller has every one of these scopes. No one (a Caller that is nobody) has none.
func (c Caller) May(scopes ...string) bool {
	return !slices.ContainsFunc(scopes, func(s string) bool { return !slices.Contains(c.Scopes, s) })
}

// SignedIn reports whether the caller is someone: a person, a program with a token, a machine.
func (c Caller) SignedIn() bool { return c.Scheme != "" }

// Auth is who may do what, for the whole app: the API's operations (Middleware) and its pages alike. An app has
// one, made from the ways in it trusts, and gives it to both:
//
//	var Auth = api.NewAuth(cloud.Var, api.People{Name: "Notes", Accounts: api.D1Accounts("DB"), Scopes: []string{"read", "write"}},
//		api.Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}})
//
//	(&app.App{Auth: Auth, Args: []any{Auth}, RequestArgs: []any{Auth.Caller}, …}).Run()
//
// A page then declares who it needs as the contract's operations do: a Needs method on its type (or on a group
// above it in the tree) with the scopes, and a caller api.Caller among its Props' parameters when it shows
// something of them.
type Auth struct {
	g      gate
	people *People
}

// NewAuth is the app's Auth: setting looks a secret or variable up, trusted are the ways in.
func NewAuth(setting func(name string) string, trusted ...Trusted) *Auth {
	g := newGate(setting, trusted)
	return &Auth{g: g, people: g.people}
}

// Caller is who the request comes from: the request arg a page's Props asks for by its type. A request with no
// credentials, or with a session that has ended, is nobody (Caller.SignedIn is false), and no error.
func (a *Auth) Caller(r *http.Request) (Caller, error) {
	if c, ok := r.Context().Value(callerKey{}).(Caller); ok {
		return c, nil
	}
	// Asked once for a request: the page's check, its Props and its header all ask.
	memo := app.Memo(r.Context())
	if memo != nil {
		if c, ok := memo.Load(callerKey{}); ok {
			return c.(Caller), nil
		}
	}
	c := a.identified(r)
	if memo != nil {
		memo.Store(callerKey{}, c)
	}
	return c, nil
}

func (a *Auth) identified(r *http.Request) Caller {
	callers, err := a.g.identify(r.Context(), r.Header.Get)
	if err != nil || len(callers) == 0 {
		return Caller{}
	}
	// Whoever may do most: a person behind Access who also signed in is both.
	best := callers[0]
	for _, c := range callers[1:] {
		if len(c.Scopes) > len(best.Scopes) {
			best = c
		}
	}
	return best
}

// For is a request that is ctx's, for an operation of the contract (which has a context, and no request) to ask
// what a page asks: auth.Everyone(auth.For(ctx)). Its caller is the one the API's middleware let in.
func (a *Auth) For(ctx context.Context) *http.Request {
	return (&http.Request{Header: http.Header{}}).WithContext(ctx)
}

// Current is who the page being rendered is for: for a component that is no page's own, such as a header that
// says who is signed in. Nobody when the app has no Auth.
func Current(ctx context.Context) Caller {
	auth, _ := app.AuthOf(ctx).(*Auth)
	r := app.Request(ctx)
	if auth == nil || r == nil {
		return Caller{}
	}
	c, _ := auth.Caller(r)
	return c
}

// Allow is the check a page's Needs gets, the one an operation's Security gets: nil when the request's caller has
// the scopes, an error with status 401 when it is nobody (which app.App turns into the way to sign in), 403 when
// it is someone who may not.
func (a *Auth) Allow(r *http.Request, needs ...string) error {
	c, _ := a.Caller(r)
	switch {
	case c.May(needs...) && (len(needs) > 0 || c.SignedIn()):
		return nil
	case !c.SignedIn():
		return app.Status(http.StatusUnauthorized, "sign in to do this")
	}
	return app.Status(http.StatusForbidden, c.Name()+" may not do this: it needs "+strings.Join(needs, " and "))
}

// PeopleScheme says in the specs that a bearer token may be one of the app's own: a token someone signed in to
// the app made for a program, on their account page. Call it after Scheme. A browser on the app's own pages needs
// none: its session is a cookie of the same origin, which is the same caller.
func PeopleScheme(spec *huma.OpenAPI) {
	if s := spec.Components.SecuritySchemes[Name]; s != nil {
		s.Description = "A token of the Worker's own, or one a person who signed in to the app made for a program on their account page: it may do what they may, or less"
	}
}

// D1Accounts keeps People in the D1 database bound as binding, in the tables charter add screen:sign-in writes
// into db/schema.sql: users, passkeys, tokens, ceremonies.
func D1Accounts(binding string) func() (Accounts, error) {
	return func() (Accounts, error) {
		db, err := cloud.Open(binding)
		return d1Accounts{db}, err
	}
}

// AccountsOn keeps People in a D1 database that is open already: for an app's seed, which is handed one.
func AccountsOn(db cloud.DB) Accounts { return d1Accounts{db} }

// SeedPerson keeps a person for an app's seed, and, when token is not empty, a token that is them: a value the seed
// names, so that a developer, a test and an agent can be that person on a database they have just seeded. A token
// starts with "ch_", as every token the app issues does. This is for a seed, which no deployed Worker has: a token
// whose value is written down is no secret.
func SeedPerson(ctx context.Context, accounts Accounts, who Person, token string) error {
	if err := accounts.Create(ctx, who); err != nil {
		return err
	}
	if token == "" {
		return nil
	}
	if !strings.HasPrefix(token, tokenPrefix) {
		return errors.New("api: a seeded token starts with " + tokenPrefix)
	}
	return accounts.Issue(ctx, who.ID, TokenScheme, "seed", hashOf(token), nil, time.Time{})
}

type d1Accounts struct{ db cloud.DB }

type personRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Scopes string `json:"scopes"`
	Kind   string `json:"kind"`
}

func (r personRow) person() Person { return Person{ID: r.ID, Name: r.Name, Role: r.Role} }

// stamp is a time as SQLite's CURRENT_TIMESTAMP writes it, so the two compare as text.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") }

func none(err error) error {
	if errors.Is(err, cloud.ErrNoRows) {
		return ErrNoAccount
	}
	return err
}

func (a d1Accounts) ByToken(_ context.Context, hash string) (Person, string, []string, error) {
	row, err := cloud.One[personRow](a.db, "SELECT users.id, users.name, users.role, tokens.scopes, tokens.kind FROM tokens JOIN users ON users.id = tokens.user_id WHERE tokens.hash = ? AND (tokens.expires_at IS NULL OR tokens.expires_at > CURRENT_TIMESTAMP)", hash)
	return row.person(), row.Kind, strings.Fields(row.Scopes), none(err)
}

func (a d1Accounts) Person(_ context.Context, id string) (Person, error) {
	row, err := cloud.One[personRow](a.db, "SELECT id, name, role FROM users WHERE id = ?", id)
	return row.person(), none(err)
}

func (a d1Accounts) Create(_ context.Context, p Person) error {
	return cloud.Exec(a.db, "INSERT INTO users (id, name, role) VALUES (?, ?, ?)", p.ID, p.Name, p.Role)
}

func (a d1Accounts) Everyone(context.Context) ([]Person, error) {
	rows, err := cloud.Query[personRow](a.db, "SELECT id, name, role FROM users ORDER BY created_at, id LIMIT 500")
	out := make([]Person, len(rows))
	for i, row := range rows {
		out[i] = row.person()
	}
	return out, err
}

func (a d1Accounts) SetRole(_ context.Context, id, role string) error {
	n, err := cloud.ExecRows(a.db, "UPDATE users SET role = ? WHERE id = ?", role, id)
	if err == nil && n == 0 {
		return ErrNoAccount
	}
	return err
}

func (a d1Accounts) Passkeys(_ context.Context, userID string) ([]string, error) {
	rows, err := cloud.Query[struct {
		Credential string `json:"credential"`
	}](a.db, "SELECT credential FROM passkeys WHERE user_id = ?", userID)
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Credential
	}
	return out, err
}

func (a d1Accounts) AddPasskey(_ context.Context, userID, id, credential string) error {
	return cloud.Exec(a.db, "INSERT INTO passkeys (id, user_id, credential) VALUES (?, ?, ?) ON CONFLICT (id) DO UPDATE SET credential = excluded.credential", id, userID, credential)
}

func (a d1Accounts) Issue(_ context.Context, userID, kind, name, hash string, scopes []string, expires time.Time) error {
	var until any
	if !expires.IsZero() {
		until = stamp(expires)
	}
	return cloud.Exec(a.db, "INSERT INTO tokens (hash, user_id, kind, name, scopes, expires_at) VALUES (?, ?, ?, ?, ?, ?)", hash, userID, kind, name, strings.Join(scopes, " "), until)
}

func (a d1Accounts) Tokens(_ context.Context, userID string) ([]IssuedToken, error) {
	rows, err := cloud.Query[struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Scopes  string `json:"scopes"`
		Created string `json:"created_at"`
	}](a.db, "SELECT id, name, scopes, created_at FROM tokens WHERE user_id = ? AND kind = ? ORDER BY id DESC", userID, TokenScheme)
	out := make([]IssuedToken, len(rows))
	for i, row := range rows {
		out[i] = IssuedToken{ID: row.ID, Name: row.Name, Scopes: strings.Fields(row.Scopes), Created: row.Created}
	}
	return out, err
}

func (a d1Accounts) Revoke(_ context.Context, userID string, id int64, hash string) error {
	return cloud.Exec(a.db, "DELETE FROM tokens WHERE user_id = ? AND (id = ? OR hash = ?)", userID, id, hash)
}

func (a d1Accounts) Begin(_ context.Context, id, data string, expires time.Time) error {
	// What was started and never finished goes as the next one starts.
	return cloud.Batch(a.db,
		cloud.Stmt("DELETE FROM ceremonies WHERE expires_at < CURRENT_TIMESTAMP"),
		cloud.Stmt("INSERT INTO ceremonies (id, data, expires_at) VALUES (?, ?, ?)", id, data, stamp(expires)))
}

func (a d1Accounts) Take(_ context.Context, id string) (string, error) {
	row, err := cloud.One[struct {
		Data string `json:"data"`
	}](a.db, "DELETE FROM ceremonies WHERE id = ? AND expires_at > CURRENT_TIMESTAMP RETURNING data", id)
	return row.Data, none(err)
}

// AccountsSchema is the tables D1Accounts keeps People in, as charter add screen:sign-in writes them into
// db/schema.sql (the one place a table is written down; charter writes the migration from there).
const AccountsSchema = `-- The app's own users (charter's sign-in screen, package api): who signed up and what they are (their role), the
-- passkeys they sign in with, the sessions and tokens they hold (never the token: its SHA-256), and sign-ins that were started.
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  role TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE passkeys (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users (id),
  credential TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  hash TEXT NOT NULL UNIQUE,
  user_id TEXT NOT NULL REFERENCES users (id),
  kind TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  scopes TEXT NOT NULL,
  expires_at TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ceremonies (
  id TEXT PRIMARY KEY,
  data TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
`

// Everyone is the app's people, for whoever may manage them (People's Manage scope): a person signed in, or a
// program with a token that has it.
func (a *Auth) Everyone(r *http.Request) ([]Person, error) {
	accounts, err := a.managing(r)
	if err != nil {
		return nil, err
	}
	return accounts.Everyone(r.Context())
}

// SetRole makes a person what role says, for whoever may manage people. What they may do changes at their next
// request, in their open sessions and for the tokens they made.
func (a *Auth) SetRole(r *http.Request, id, role string) error {
	accounts, err := a.managing(r)
	if err != nil {
		return err
	}
	if _, ok := a.people.Roles[role]; !ok {
		return app.Status(http.StatusUnprocessableEntity, "no such role: "+role)
	}
	if err := accounts.SetRole(r.Context(), id, role); errors.Is(err, ErrNoAccount) {
		return app.NotFound("no such person")
	} else if err != nil {
		return err
	}
	return nil
}

// Roles are the roles there are, in order of their names: what a page that changes someone's offers.
func (a *Auth) Roles() []string {
	if a.people == nil {
		return nil
	}
	names := make([]string, 0, len(a.people.Roles))
	for name := range a.people.Roles {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (a *Auth) managing(r *http.Request) (Accounts, error) {
	if a.people == nil {
		return nil, errors.New("api: the app's Auth has no People")
	}
	if err := a.Allow(r, a.people.manage()); err != nil {
		return nil, err
	}
	return a.people.Accounts()
}
