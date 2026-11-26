-- The database, as it is now: the one place a table is written down. charter writes everything else from it: the
-- migrations (db/migrations, for whatever changes here), and with the queries of queries.sql the typed Go
-- (queries.gen.go). A column's comment says what the API shows of it: its row is also what the contract serves.

CREATE TABLE notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,                 -- example: 42
  body TEXT NOT NULL,                                   -- example: Buy milk
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP    -- example: 2026-10-01 12:00:00
);
