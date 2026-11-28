//go:build js && wasm

package cloud

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"

	"github.com/joeblew999/charter/internal/promise"
)

// DB is a D1 database, by its binding's name. A statement on it runs in the session of the request being served
// (session.go): there is no connection to open or close, and a DB that is kept from one request to the next is the
// next request's.
type DB struct{ name string }

// Open is the database bound as name (wrangler.jsonc's d1_databases).
func Open(name string) (DB, error) {
	if !cloudflare.GetBinding(name).Truthy() {
		return DB{}, fmt.Errorf("%s is not bound (wrangler.jsonc's d1_databases)", name)
	}
	return DB{name}, nil
}

// serving is the request this Go runtime is serving, which is one at a time (app/worker/go.mjs): its sessions, one
// for each database it has used. nil outside a request.
var serving *request

type request struct {
	brought  Bookmarks           // what the request brought: where its sessions start
	newest   bool                // Newest was called: they start at the primary
	sessions map[string]*session // by the binding's name
	reached  Bookmarks           // after each statement: where the database it ran on then was, as this request saw it
	served   []string            // who answered each statement
}

// session is a D1 session and where it started.
type session struct {
	value js.Value
	start string
	ran   bool // a statement has run in it
}

// Begin starts a request: its statements share a session for each database, which starts at what the request
// brought. app.Serve calls it; an app never does.
func Begin(brought Bookmarks) {
	serving = &request{brought: brought, sessions: map[string]*session{}, reached: Bookmarks{}}
}

// End ends the request Begin started.
func End() { serving = nil }

// Newest has the rest of the request read what is newest: its sessions start at the primary, and one that has
// started elsewhere is left for a new one that does. For a read that must not be behind: who a caller is, and
// what a feed has just told a reader of. Outside a request it does nothing, since every statement there starts at
// the primary.
func Newest() {
	if serving == nil {
		return
	}
	serving.newest = true
	for name, s := range serving.sessions {
		if s.start != fromPrimary {
			delete(serving.sessions, name)
		}
	}
}

// Brought is the bookmarks the request being served brought. None outside a request.
func Brought() Bookmarks {
	if serving == nil {
		return nil
	}
	return serving.brought
}

// Reached is where each database was as the request last saw it, read or written: what the response gives the
// reader to bring back, so that their next request sees nothing older than this one did. Empty for a request that
// ran no statement.
func Reached() Bookmarks {
	if serving == nil {
		return nil
	}
	return serving.reached.Join(nil)
}

// Served is who answered each of the request's statements, in order: "DB APAC primary", "DB WEUR replica".
func Served() []string {
	if serving == nil {
		return nil
	}
	return serving.served
}

// in is the session a statement on db runs in: the request's, started on its first use, or one of its own at the
// primary outside a request.
func (db DB) in() *session {
	if serving == nil {
		return db.started(startOf(false, false, ""))
	}
	if s := serving.sessions[db.name]; s != nil {
		return s
	}
	s := db.started(startOf(true, serving.newest, serving.brought[db.name]))
	serving.sessions[db.name] = s
	return s
}

// started is a new session on db from start. A start D1 refuses on the spot is replaced by the primary.
func (db DB) started(start string) (s *session) {
	binding := cloudflare.GetBinding(db.name)
	defer func() {
		if recover() != nil {
			s = &session{value: binding.Call("withSession", fromPrimary), start: fromPrimary}
		}
	}()
	return &session{value: binding.Call("withSession", start), start: start}
}

// run runs one call on db's session and waits for it. A session that started from a bookmark the request brought
// and fails at its first statement starts again at the primary, once: a bookmark of another database, or one D1 no
// longer keeps, costs the request nothing but that.
func (db DB) run(call func(session js.Value) js.Value) (js.Value, error) {
	s := db.in()
	result, err := awaited(s, call)
	if err != nil && !s.ran && s.start != fromPrimary && s.start != fromAnywhere {
		s = db.started(fromPrimary)
		if serving != nil {
			serving.sessions[db.name] = s
		}
		result, err = awaited(s, call)
	}
	if err != nil {
		return js.Undefined(), fmt.Errorf("d1: %w", err)
	}
	s.ran = true
	db.noted(s, result)
	return result, nil
}

// awaited is call's promise waited for. What JavaScript throws before there is a promise is an error too.
func awaited(s *session, call func(session js.Value) js.Value) (result js.Value, err error) {
	defer func() {
		if thrown := recover(); thrown != nil {
			result, err = js.Undefined(), fmt.Errorf("%v", thrown)
		}
	}()
	return promise.Await(call(s.value))
}

// noted keeps what a statement's answer says for the request: who answered, and where the database then was.
// result is a statement's (D1Result) or a batch's (an array of them).
func (db DB) noted(s *session, result js.Value) {
	if serving == nil {
		return
	}
	note := func(one js.Value) {
		meta := one.Get("meta")
		if meta.Type() != js.TypeObject {
			return
		}
		// Under wrangler dev there is one database and no region.
		who := "local"
		if region := meta.Get("served_by_region"); region.Type() == js.TypeString {
			who = region.String() + " replica"
			if meta.Get("served_by_primary").Truthy() {
				who = region.String() + " primary"
			}
		}
		serving.served = append(serving.served, db.name+" "+who)
	}
	if result.InstanceOf(js.Global().Get("Array")) {
		for i := 0; i < result.Length(); i++ {
			note(result.Index(i))
		}
	} else if result.Type() == js.TypeObject {
		note(result)
	}
	// After a read as after a write: a reader who was shown the database at one point is never shown it earlier.
	if bookmark := s.value.Call("getBookmark"); bookmark.Type() == js.TypeString {
		serving.reached[db.name] = Newer(serving.reached[db.name], bookmark.String())
	}
}

var stringify = js.Global().Get("JSON").Get("stringify")

// Query runs one statement and returns its rows: those of a SELECT or of a RETURNING clause, and
// none for any other statement. A row is decoded from JSON, an object with the column names as its
// keys, so T is a struct whose json tags name the columns. An INTEGER or a REAL is a number, TEXT
// a string, NULL null, and a BLOB an array of numbers.
//
// The arguments are bound to the statement's ? in order: strings, numbers, bools and nil.
func Query[T any](db DB, query string, args ...any) ([]T, error) {
	result, err := db.run(func(session js.Value) js.Value {
		return session.Call("prepare", query).Call("bind", args...).Call("all")
	})
	if err != nil {
		return nil, err
	}
	rows := []T{}
	if err := json.Unmarshal([]byte(stringify.Invoke(result.Get("results")).String()), &rows); err != nil {
		return nil, fmt.Errorf("d1: a row is not a %T: %w", *new(T), err)
	}
	return rows, nil
}

// Exec runs one statement whose rows, if it has any, are not wanted.
func Exec(db DB, query string, args ...any) error {
	_, err := db.run(func(session js.Value) js.Value {
		return session.Call("prepare", query).Call("bind", args...).Call("run")
	})
	return err
}

// ExecRows runs one statement and returns how many rows it changed (an INSERT, UPDATE or DELETE): D1's meta.changes.
func ExecRows(db DB, query string, args ...any) (int64, error) {
	result, err := db.run(func(session js.Value) js.Value {
		return session.Call("prepare", query).Call("bind", args...).Call("run")
	})
	if err != nil {
		return 0, err
	}
	return int64(result.Get("meta").Get("changes").Int()), nil
}

// Batch runs the statements in order as one transaction, D1's batch: one call, and if a statement fails, none of
// them happened. No statements is no call: D1 refuses an empty batch.
func Batch(db DB, statements ...Statement) error {
	if len(statements) == 0 {
		return nil
	}
	_, err := db.run(func(session js.Value) js.Value {
		prepared := make([]any, len(statements))
		for i, s := range statements {
			prepared[i] = session.Call("prepare", s.Query).Call("bind", s.Args...)
		}
		return session.Call("batch", js.ValueOf(prepared))
	})
	if err != nil {
		return fmt.Errorf("d1: batch: %w", errUnwrapped(err))
	}
	return nil
}

// errUnwrapped is err without run's own "d1: ", for a caller that says more.
func errUnwrapped(err error) error {
	if inner, ok := err.(interface{ Unwrap() error }); ok && inner.Unwrap() != nil {
		return inner.Unwrap()
	}
	return err
}
