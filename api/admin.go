package api

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/internal/accounts"
)

// What an admin sees of the system: who there is, what the Worker is built from and bound to, and what its
// databases hold. Each is for whoever may manage people (People's Manage scope), asked of the Worker as it runs, so
// a page and a program are told the same thing. Nothing here changes anything, and no secret's value is ever given.

// Overview is the app at a glance.
type Overview struct {
	App string `json:"app"`
	// Charter and Go are the versions the Worker was built with; Revision and Committed are the app's own commit
	// (empty for a build outside a repository), and Modified says the build had changes that were not committed.
	Charter   string    `json:"charter"`
	Go        string    `json:"go"`
	Revision  string    `json:"revision,omitempty"`
	Committed time.Time `json:"committed,omitzero"`
	Modified  bool      `json:"modified,omitempty"`
	// People is how many there are of each role, in the order of the roles' names.
	People []RoleCount `json:"people"`
	// Invitations are those that can still be taken up.
	Invitations []Invitation `json:"invitations"`
}

// RoleCount is how many people have a role.
type RoleCount struct {
	Role  string `json:"role"`
	Count int    `json:"count"`
}

// Invitation is an invitation that can still be taken up: what it is to, and until when. ID is what it is kept
// under (never its code, which only its link has), for cancelling it.
type Invitation struct {
	ID      string    `json:"id"`
	Role    string    `json:"role"`
	Expires time.Time `json:"expires"`
}

// Overview is the app at a glance, for whoever may manage people.
func (a *Auth) Overview(r *http.Request) (Overview, error) {
	accounts, err := a.managing(r)
	if err != nil {
		return Overview{}, err
	}
	o := Overview{App: app.Name(r.Context()), People: []RoleCount{}, Invitations: []Invitation{}}
	if info, ok := debug.ReadBuildInfo(); ok {
		o.Go = info.GoVersion
		for _, dep := range info.Deps {
			if dep.Path == "github.com/joeblew999/charter" {
				o.Charter = dep.Version
			}
		}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				o.Revision = s.Value
			case "vcs.time":
				o.Committed, _ = time.Parse(time.RFC3339, s.Value)
			case "vcs.modified":
				o.Modified = s.Value == "true"
			}
		}
	}
	everyone, err := accounts.Everyone(r.Context())
	if err != nil {
		return o, err
	}
	for _, role := range a.Roles() {
		n := 0
		for _, p := range everyone {
			if p.Role == role {
				n++
			}
		}
		o.People = append(o.People, RoleCount{role, n})
	}
	if o.Invitations, err = accounts.Invitations(r.Context()); err != nil {
		return o, err
	}
	if o.Invitations == nil {
		o.Invitations = []Invitation{}
	}
	return o, nil
}

// CancelInvitation takes an invitation away before anyone takes it up, by the id Overview gave it.
func (a *Auth) CancelInvitation(r *http.Request, id string) error {
	accounts, err := a.managing(r)
	if err != nil {
		return err
	}
	if _, err := accounts.Take(r.Context(), invitePrefix+id); err != nil {
		return app.NotFound("no such invitation: it was taken up, cancelled, or is too old")
	}
	return nil
}

// System is what the Worker is bound to, as it runs: its bindings by kind, and its variables and secrets by name.
type System struct {
	Bindings []cloud.Binding `json:"bindings"`
	Settings []cloud.Binding `json:"settings"`
	// Memory is what this instance's Go runtimes hold: nil when the Worker's entry does not say.
	Memory *Memory `json:"memory,omitempty"`
}

// Memory is the Go runtimes of one instance of the Worker: how many are alive, what they hold between them and
// what they may, in bytes. A Worker has 128 MB for everything; Cloudflare ends one that takes more.
type Memory struct {
	Runtimes int   `json:"runtimes"`
	Held     int64 `json:"held"`
	Budget   int64 `json:"budget"`
}

// System is what the Worker is bound to, for whoever may manage people.
func (a *Auth) System(r *http.Request) (System, error) {
	if _, err := a.managing(r); err != nil {
		return System{}, err
	}
	s := System{Bindings: []cloud.Binding{}, Settings: []cloud.Binding{}}
	if count, held, budget, ok := cloud.Runtimes(); ok {
		s.Memory = &Memory{count, held, budget}
	}
	for _, b := range cloud.Bindings() {
		if b.Kind == cloud.KindText {
			s.Settings = append(s.Settings, b)
		} else {
			s.Bindings = append(s.Bindings, b)
		}
	}
	return s, nil
}

// Probed is a binding's answer to one small question: whether it answered, how long it took, and what it said when
// it did not. Asked false: a binding of a kind that has nothing harmless to ask.
type Probed struct {
	Binding string        `json:"binding"`
	Asked   bool          `json:"asked"`
	OK      bool          `json:"ok"`
	Took    time.Duration `json:"took"`
	Error   string        `json:"error,omitempty"`
}

// Probe asks one binding one small thing (cloud.Probe), for whoever may manage people.
func (a *Auth) Probe(r *http.Request, binding string) (Probed, error) {
	if _, err := a.managing(r); err != nil {
		return Probed{}, err
	}
	if !slices.ContainsFunc(cloud.Bindings(), func(b cloud.Binding) bool { return b.Name == binding && b.Kind != cloud.KindText }) {
		return Probed{}, app.NotFound("no such binding")
	}
	took, err := cloud.Probe(binding)
	switch {
	case errors.Is(err, cloud.ErrNoProbe):
		return Probed{Binding: binding}, nil
	case err != nil:
		return Probed{Binding: binding, Asked: true, Took: took, Error: err.Error()}, nil
	}
	return Probed{Binding: binding, Asked: true, OK: true, Took: took}, nil
}

// Database is one database of the Worker: its tables with how many rows each has, and the migrations that ran.
type Database struct {
	Binding    string      `json:"binding"`
	Tables     []Table     `json:"tables"`
	Migrations []Migration `json:"migrations"`
}

// Table is a table and how many rows it has.
type Table struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
}

// Migration is a migration that ran, and when.
type Migration struct {
	Name    string    `json:"name"`
	Applied time.Time `json:"applied"`
}

// Databases are the Worker's databases, for whoever may manage people.
func (a *Auth) Databases(r *http.Request) ([]Database, error) {
	if _, err := a.managing(r); err != nil {
		return nil, err
	}
	out := []Database{}
	for _, b := range cloud.Bindings() {
		if b.Kind != cloud.KindDatabase {
			continue
		}
		// A name the Worker itself gave, as it runs: the one place a binding is made from a name that is not written
		// in wrangler.jsonc's own words.
		db, err := cloud.BoundDatabase(b.Name).Open()
		if err != nil {
			return nil, err
		}
		d := Database{Binding: b.Name, Tables: []Table{}, Migrations: []Migration{}}
		// The three statements here are the only ones charter writes in Go, and cannot be otherwise: they ask what
		// tables a database has (SQLite's own catalogue), count one by a name read from there, and read wrangler's
		// record of its migrations. None is a table of a schema, so there is nothing to type them against.
		tables, err := cloud.Query[struct {
			Name string `json:"name"`
		}](db, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_cf_%' AND name <> 'd1_migrations' ORDER BY name")
		if err != nil {
			return nil, err
		}
		for _, t := range tables {
			// A table's name is the database's own word for it, quoted as one.
			n, err := cloud.One[struct {
				N int64 `json:"n"`
			}](db, `SELECT count(*) AS n FROM "`+strings.ReplaceAll(t.Name, `"`, `""`)+`"`)
			if err != nil {
				return nil, err
			}
			d.Tables = append(d.Tables, Table{t.Name, n.N})
		}
		// wrangler's own record of the migrations it applied; a database made another way has none.
		ran, err := cloud.Query[struct {
			Name    string `json:"name"`
			Applied string `json:"applied_at"`
		}](db, "SELECT name, applied_at FROM d1_migrations ORDER BY id")
		if err == nil {
			for _, m := range ran {
				at, _ := time.Parse("2006-01-02 15:04:05", m.Applied)
				d.Migrations = append(d.Migrations, Migration{m.Name, at})
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// invitations are the invitations a store of started sign-ins holds, from its rows.
func invitations(rows []accounts.CeremoniesRow) []Invitation {
	out := []Invitation{}
	for _, row := range rows {
		expires, _ := time.Parse("2006-01-02 15:04:05", row.ExpiresAt)
		out = append(out, Invitation{ID: strings.TrimPrefix(row.ID, invitePrefix), Role: row.Data, Expires: expires})
	}
	return out
}

func (a d1Accounts) Invitations(context.Context) ([]Invitation, error) {
	rows, err := accounts.StartedLike(a.db, invitePrefix+"%")
	return invitations(rows), err
}
