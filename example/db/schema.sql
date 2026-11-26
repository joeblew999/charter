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
