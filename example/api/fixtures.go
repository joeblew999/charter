package api

import (
	"context"

	charter "github.com/joeblew999/charter/api"
)

// Seeded is what a new checkout's database starts with, and what a test knows is there: a few people, one of each
// role, and notes they wrote. It is written down once, as values of the app's own types. The seed fills the local
// database with it (go tool charter run db:seed; charter dev, on a database it has just made; charter test, on the
// one it makes), and a test of a page fills its own store with the same call, so both show the same notes by the
// same people.
//
// Each person has a token, so that a developer, a test or an agent can be the admin, the moderator or the member
// at once: Authorization: Bearer ch_seed_grace. The seed is built only for seeding and runs only on this machine;
// a deployed Worker has neither these people nor their tokens.
var Seeded = struct {
	Ada, Grace, Lin SeededPerson
	Notes           []SeededNote
}{
	Ada:   SeededPerson{charter.Person{ID: "seed-ada", Name: "Ada", Role: "admin"}, "ch_seed_ada"},
	Grace: SeededPerson{charter.Person{ID: "seed-grace", Name: "Grace", Role: "moderator"}, "ch_seed_grace"},
	Lin:   SeededPerson{charter.Person{ID: "seed-lin", Name: "Lin", Role: "member"}, "ch_seed_lin"},
}

func init() {
	s := &Seeded
	s.Notes = []SeededNote{
		{s.Lin, "Lin here. A member writes notes, and takes away their own."},
		{s.Grace, "Grace, moderating: I can take away anyone's note."},
		{s.Ada, "Ada, the admin: I say who is what, on the people page."},
		{s.Lin, "Sign up with a passkey and write one yourself."},
	}
}

// SeededPerson is one of the seed's people, and the token that is them.
type SeededPerson struct {
	charter.Person
	Token string
}

// Caller is the person as the app sees them when they ask: who a test hands a page, in place of signing in.
func (p SeededPerson) Caller() charter.Caller {
	return charter.Caller{Scheme: charter.TokenScheme, Subject: p.ID, Person: p.Name, Role: p.Role, Scopes: Roles[p.Role]}
}

// SeededNote is one of the seed's notes: who wrote it, and what it says.
type SeededNote struct {
	By   SeededPerson
	Body string
}

// Seed fills a store with Seeded, the way the app itself writes: each note through Post, by its writer, as the
// page's form and the API's createNote write one. accounts is where the people are kept (the Worker's database);
// a test of a page, which is handed its caller, passes nil.
func Seed(ctx context.Context, store Store, accounts charter.Accounts) error {
	if accounts != nil {
		for _, p := range []SeededPerson{Seeded.Ada, Seeded.Grace, Seeded.Lin} {
			if err := charter.SeedPerson(ctx, accounts, p.Person, p.Token); err != nil {
				return err
			}
		}
	}
	for _, n := range Seeded.Notes {
		if _, err := Post(ctx, store, n.By.Caller(), n.Body); err != nil {
			return err
		}
	}
	return nil
}
