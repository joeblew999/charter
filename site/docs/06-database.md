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

A seed is what a new checkout's database starts with. It is Go, with the app's own queries, so a column that changes breaks it when it is compiled, not when someone needs it.

```go file=../example/db/seed.go#seed
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
```

```go
//go:build seed

func init() { app.Seed("DB", seed) }
```

```sh
go tool charter run db:seed
```

That makes the local database again (its rows gone, the migrations run) and fills it. `charter dev` does the same for a database it has just made, so the first page has something on it; a database that is there is yours, and is left alone.

The seed runs where the app's database code always runs, in a Worker: charter starts one here, built with the seed in it, asks it to run, and stops it. A Worker that is deployed has none of it, and `db:seed` refuses a deployed database. `db/seed.sql` works too, for plain SQL.

## By hand

`go tool charter run db:status`, `db:execute "…"`, `db:export`: each for the local database, or the deployed one with `--deployed`. `db:seed` and `db:reset` make the local database again, and only that one.
