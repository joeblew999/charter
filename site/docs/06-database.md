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

### Every statement is checked

sqlc works out each query against `db/schema.sql`: a column that is not there, an argument or a result of the wrong type, fails `charter check`, and the Go that calls the function with the wrong types does not compile. So what is checked is every statement, as long as every statement is a query. `charter check` holds an app to that (its part `statements`):

- **No statement is written in Go.** A call that hands the database SQL as text (`cloud.Query`, `cloud.One`, `cloud.Exec`, `cloud.ExecRows`, `cloud.Stmt`, a `cloud.Statement{…}` written out) outside `db/queries.gen.go` fails the check, with the file and the line. Those functions are what the generated code is made of, not something to call.
- **Several statements in one transaction** are `cloud.Batch` of the `…Stmt` function charter writes beside each query that returns no rows:

  ```go
  forget := make([]cloud.Statement, len(gone))
  for i, note := range gone {
  	forget[i] = db.ForgetNoteStmt(note.ID)
  }
  return cloud.Batch(s.db, forget...)
  ```

- **Nothing reaches D1 around charter.** workers-go's `database/sql` driver (`workers-go/cloudflare/d1`) is checked by nothing and is in no request's session, so its reads are the primary's and its writes give no bookmark ([reads near the reader](#reads-near-the-reader)). Importing it fails the check.

charter holds itself to the same. The statements of its sign-in (the tables `users`, `passkeys`, `tokens`, `ceremonies`) are a `queries.sql` of its own, written as Go by the same generator, and a test fails charter if any statement in its library is written in Go. An app owns its copy of those tables, in its own `db/schema.sql`, and may add to them; `charter check` runs charter's statements against the app's schema, so a column of theirs that was dropped, renamed or retyped is said there:

```
✗ statements: db/schema.sql no longer has the tables of the sign-in screen (users, passkeys, tokens, ceremonies) as charter reads and writes them:
  queries.sql:8:30: column "role" does not exist
```

What is not checked this way, and cannot be: the admin screen asks a database what tables it has and how many rows each holds, which is SQLite's own catalogue and no table of a schema; and inside `cloud`, Go calls Cloudflare's D1 by name, which only running on a Worker proves, as the tests do.

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
// at once: Authorization: Bearer ch_seed_grace. The seed is built only for seeding and runs only on this machine.
// A deployed database filled by it (go tool charter run db:seed --deployed) has the people and their notes, and
// none of the tokens: their values are written here, for anyone to read.
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
| `charter run db:seed --deployed my-worker` | Every row of the deployed database is deleted, and the seed's put there |

So a page test, a browser test and your own screen show the same notes by the same people.

Each seeded person has a token (`ch_seed_ada`, `ch_seed_grace`, `ch_seed_lin`), so you, a test or a coding agent can be the admin, the moderator or the member at once, with no one to sign up first:

```sh
curl -H "Authorization: Bearer ch_seed_grace" -X DELETE http://localhost:5173/api/notes/1
```

What keeps this safe:

- **The seed never ships.** It is built only when charter seeds, and `charter deploy` builds without it whatever the environment says. A deployed Worker never has the seed in it, and a deployed database never has a seeded token.
- **Nothing is dropped for a seed that does not build.** It is compiled first, and `check` compiles it on every push.
- **The local database, unless you say otherwise twice.** `db:seed` alone fills the local one and refuses a flag it does not know. The deployed one needs `--deployed` and then the Worker's name ([below](#a-deployed-database)).
- **It runs where the app's database code always runs:** in a Worker charter starts here, on the real binding. Beside a running `charter dev` it works in place, with no restart.

`db/seed.sql` works too, for plain SQL, in place of `seed.go`.

### A deployed database

A demo, or a staging environment, shows the seed's data too. The same seed fills it:

```sh
go tool charter run db:seed --deployed             # what the deployed database has, table by table; changes nothing
go tool charter run db:seed --deployed my-worker   # every row deleted, then the seed's rows
```

It deletes everything, so it is asked for twice: the second time with the Worker's name. The tables stay as the migrations made them, and the deployed Worker keeps answering.

The seed still runs on your machine: a Worker started here, built with the seed, whose database binding is the deployed database. The deployed Worker never has the seed in it. What is safe only locally stays out: `api.SeedPerson` keeps the person and no token, because a token whose value is in your code is no secret. `app.SeedIsDeployed(ctx)` tells your own seed the same.

## Reads near the reader

A D1 database takes its writes in one place, its primary, and can keep a copy in each of [its regions](https://developers.cloudflare.com/d1/best-practices/read-replication/). A read answered by the copy nearest the reader spares them the trip to the primary. A copy may be a moment behind, so two things have to hold, and charter holds them for every app, with no line of the app's:

- **The statements of one request agree with each other.** They share one [session](https://developers.cloudflare.com/d1/worker-api/d1-database/#withsession), so a later one never sees less than an earlier one did.
- **A reader sees their own writes, and never the database earlier than they have seen it.** The answer to a request that read or changed the database gives a bookmark, the point the database was at as that request saw it, and the reader's next request brings it back: its reads are at least that new. So what someone wrote is there on the next page, and so is what they were shown: a note that arrived in a live list is not gone a moment later because another copy answered. A browser carries it in a cookie (`d1`, for five minutes, not readable by scripts), so it survives the redirect after a form; a program carries it in a header (`X-D1-Bookmark`), which it sends back as it was given.

`go tool charter deploy` turns the copies on for every database the Worker is bound to (D1's read replication, which wrangler has no setting for): a new app has them from its first deploy, an older one from its next. It costs nothing more: D1 bills the rows a statement reads and writes, with copies or without.

Where a request's reads start:

| The request | Its reads start |
|---|---|
| Says who it is (a session, a token), or signs someone in | At the primary. A session or a token that was just taken away, or a role that just changed, is never accepted by a copy that is behind. The rest of the request reads from there on, so it sees whatever its caller wrote before, bookmark or none |
| Calls the API and brings no bookmark | At the primary: a program that writes and then reads sees what it wrote, though its client knows nothing of bookmarks. One that sends back the `X-D1-Bookmark` it was given reads near itself |
| Is a part of a page asking for itself because its feed said something changed (`live.Follows`), or sends `X-D1-Newest` | At the primary: a copy that is behind would show the page as it was before the change |
| Brings a bookmark | At least where the bookmark says |
| Anything else | At the nearest copy |
| No request at all: a Cron Trigger, a queue's batch, an email, work left for after the answer | At the primary, each statement |

What to know:

- **A request is given a bookmark only when it saw something new.** One that ran no statement is given nothing, and so is one whose reader already holds where the database is: a page that touches no database sets no cookie.
- **A read that must be the newest says so:** `cloud.Newest()` before it. It is the one line an app may need, after a write that someone else made and this reader was told of some other way.
- **The bookmark is given as the answer's headers go.** A statement that runs after that (in a stream, or in work left for after the answer) is not in it.
- **What is brought as a bookmark and is none costs nothing.** One that is malformed starts nothing; one D1 does not know is given up for the primary.
- **Nothing reads D1 another way.** workers-go's own `database/sql` driver knows no session: what went through it would be answered by the primary and give no bookmark. `charter check` fails an app that imports it.

Who answered is said to a request that asks which runtime answered, as `charter run cold` does:

```sh
curl -sD - -o /dev/null -H 'x-go-runtime: ?' https://my-app.example.com/ | grep -i x-d1-served
```

```
x-d1-served: DB WEUR replica, DB WEUR replica
```

Which copy answers, or whether the primary does, is D1's choice, and nothing a Worker does changes it. Cloudflare says a copy is [active or inactive by the traffic it gets](https://developers.cloudflare.com/changelog/post/2025-04-10-d1-read-replication-beta/), and that reads are sent by its own measure of what is fastest. What was seen of the example (primary in APAC), a page with one read, asked with `go tool charter run cold`:

| When | Reads by a new instance of the Worker | Reads by a running one |
|---|---|---|
| The day the copies were turned on, minutes after a deploy, from 123 countries | 20 of 110 at a copy | 11 of 12 at a copy |
| Later that day, the database an hour unused, from 4 countries | 4 of 8 at a copy | |
| Minutes after that, from 12 countries | 18 of 23 at a copy | 9 of 9 at a copy |

So a copy that has had no reads for a while is not there for the first of them, and the primary answers until it is. A new instance of the Worker reads a copy as a running one does: Los Angeles read the copy in WNAM on a new instance three times in three. An app with readers in a region has that region's copy; an app nobody is using has the primary, and its first readers the trip to it.

## By hand

`go tool charter run db:status`, `db:execute "…"`, `db:export`: each for the local database, or the deployed one with `--deployed`. `db:reset` makes the local database again, and only that one; `db:seed` does too, and fills a deployed one only as [above](#a-deployed-database).
