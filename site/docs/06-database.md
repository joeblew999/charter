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

## By hand

`go tool charter run db:status`, `db:execute "…"`, `db:reset`, `db:seed`, `db:export`. Each is for the local database, or the deployed one with `--deployed`.
