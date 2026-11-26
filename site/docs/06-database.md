# The database

`db/schema.sql` is the one place a table is written down.

```sql file=../example/db/schema.sql
-- The database, as it is now: the one place a table is written down. charter writes everything else from it: the
-- migrations (db/migrations, for whatever changes here), and with the queries of queries.sql the typed Go
-- (queries.gen.go). A column's comment says what the API shows of it: its row is also what the contract serves.

CREATE TABLE notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,                 -- example: 42
  body TEXT NOT NULL,                                   -- example: Buy milk
  author_id TEXT NOT NULL DEFAULT '',                   -- example: 4f1c9a0e7b2d4c6f8a1b3c5d7e9f0a1b; doc: Who wrote it: their id, empty for a program with one of the Worker's own tokens
  author TEXT NOT NULL DEFAULT '',                      -- example: Ada; doc: What they call themselves
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP    -- example: 2026-10-01 12:00:00
);

-- What happened, as the app records it: a note written (the queue's consumer writes the row, a moment after the
-- note), and once a day how many there were (the Cron Trigger's task). The activity page follows this table live.
CREATE TABLE activity (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL,
  note_id INTEGER NOT NULL DEFAULT 0,
  author TEXT NOT NULL DEFAULT '',
  notes INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- A file someone attached: where it is in the bucket (R2 keeps the file, this table what is known of it), and the
-- note it goes with, 0 for none.
CREATE TABLE attachments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  object TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  size INTEGER NOT NULL,
  note_id INTEGER NOT NULL DEFAULT 0,
  author_id TEXT NOT NULL,
  author TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- The app's own users (charter's sign-in screen, package api): who signed up, the passkeys they sign in with, the
-- sessions and tokens they hold (never the token: its SHA-256), and sign-ins that were started.
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  role TEXT NOT NULL, -- was: scopes
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
```

## Changing it

Edit the schema and save. Nobody writes a migration.

| When | What happens |
|---|---|
| `charter dev` is running | the local database gets exactly the difference, and keeps its rows |
| `charter check --fix` | the migration for the change is written to `db/migrations` |
| `charter check` | fails while the schema is ahead of the migrations |
| `charter deploy` | applies it |

A column added at the end becomes `ALTER TABLE … ADD COLUMN`. Anything else about a table makes the table again and copies its rows. A renamed column says what it was:

```sql
title TEXT NOT NULL, -- was: name
```

A change that would lose data is refused until you ask for it: `charter check --fix --drop`.

## Queries

`db/queries.sql` holds the queries. charter writes each as a typed Go function in `db/queries.gen.go`.

```sql
-- name: CreateNote :one
INSERT INTO notes (body) VALUES (?) RETURNING *;
```

Select whole rows, and a new column needs no change here.

## One definition

A column's comment becomes its tag in Go, so the row is also what the API serves.

```go file=../example/api/contract.go#Note
// Note is a note as the API shows it: the table's row, as db/schema.sql has it (db.NotesRow, whose examples are the
// schema's comments). A note is written down once, in the schema.
type Note struct{ db.NotesRow }
```

## Seeding

A seed is what a new checkout's database starts with. In charter it is also what the tests know is there: one set of values, written once, in the app's own types.

```go file=../example/api/fixtures.go#Seeded
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
```

It fills a store the way the app itself writes, through the app's own functions, so a seeded note is a note the app would have made: with its writer, and announced to whoever follows.

```go file=../example/api/fixtures.go#Seed
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
```

`seed.go`, at the app's top, tells charter about it. It is built only for seeding:

```go file=../example/seed.go#init
// The seed: what fills the local database (go tool charter run db:seed; charter dev and charter test, on a database
// they have just made). What it fills it with is api.Seeded, written with the app's own types and through its own
// functions; this file only says so to charter. It is built only for seeding (the build tag), in a Worker started
// on this machine: a Worker that is deployed has none of it.
func init() {
	app.Seed("DB", func(ctx context.Context, conn cloud.DB) error {
		// What the app does with each note written, it does for a seeded one: here at once, where a request would
		// leave it to the queue, so the activity page has the seed's notes on it when the seed is done.
		queued := api.OnPost
		api.OnPost = func(ctx context.Context, note api.Note) {
			built.Record(ctx, conn, built.NoteWritten, note.ID, note.Author, 0)
		}
		defer func() { api.OnPost = queued }()
		return api.Seed(ctx, api.StoreOn(conn), charter.AccountsOn(conn))
	})
}
```

```sh
go tool charter run db:seed
```

| When | What happens |
|---|---|
| `charter run db:seed` | The local database is made again (its tables gone, the migrations run) and filled |
| `charter dev`, the first time | The database it has just made is filled; one that is there is yours, and is left alone |
| `charter test` | The Worker under test starts on a database of its own, filled the same way |
| A test of a page, as plain Go | Calls `api.Seed` on its own store: the same people and notes, in memory |

So a page test, a browser test and your own screen show the same notes by the same people.

Each seeded person has a token (`ch_seed_ada`, `ch_seed_grace`, `ch_seed_lin`), so you, a test or a coding agent can be the admin, the moderator or the member at once, with no one to sign up first:

```sh
curl -H "Authorization: Bearer ch_seed_grace" -X DELETE http://localhost:5173/api/notes/1
```

What keeps this safe:

- **The seed never ships.** It is built only when charter seeds, and `charter deploy` builds without it whatever the environment says. A deployed Worker has neither the people nor their tokens.
- **Nothing is dropped for a seed that does not build.** It is compiled first, and `check` compiles it on every push.
- **Only the local database.** `db:seed` takes no flag, and refuses one it does not know.
- **It runs where the app's database code always runs:** in a Worker charter starts here, on the real binding. Beside a running `charter dev` it works in place, with no restart.

`db/seed.sql` works too, for plain SQL, in place of `seed.go`.

## By hand

`go tool charter run db:status`, `db:execute "…"`, `db:export`: each for the local database, or the deployed one with `--deployed`. `db:seed` and `db:reset` make the local database again, and only that one.
