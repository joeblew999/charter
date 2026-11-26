//go:build seed

package db

import (
	"context"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/cloud"
)

// What a new checkout's database starts with (go tool charter run db:seed; charter dev, on a database it has just
// made): a few people, one of each role, and notes they wrote. So the first page a developer opens shows who wrote
// what, and signing up beside them shows what a member may and may not do.
//
// It is Go, with the app's own queries: a column that changes breaks it when it is compiled (charter check), not
// when someone needs it. It is built only for seeding, and runs in a Worker on this machine: a deployed Worker has
// none of it. The people here have no passkey, so nobody signs in as them.
func init() { app.Seed("DB", seed) }

func seed(_ context.Context, conn cloud.DB) error {
	people := []struct{ id, name, role string }{
		{"seed-ada", "Ada", "admin"},
		{"seed-grace", "Grace", "moderator"},
		{"seed-lin", "Lin", "member"},
	}
	for _, p := range people {
		// The people's table is the sign-in screen's, with no query of the app's own.
		if err := cloud.Exec(conn, "INSERT INTO users (id, name, role) VALUES (?, ?, ?)", p.id, p.name, p.role); err != nil {
			return err
		}
	}
	notes := []struct {
		by   int
		body string
	}{
		{2, "Lin here. A member writes notes, and takes away their own."},
		{1, "Grace, moderating: I can take away anyone's note."},
		{0, "Ada, the admin: I say who is what, on the people page."},
		{2, "Sign up with a passkey and write one yourself."},
	}
	for _, n := range notes {
		if _, err := CreateNote(conn, n.body, people[n.by].id, people[n.by].name); err != nil {
			return err
		}
	}
	return nil
}
