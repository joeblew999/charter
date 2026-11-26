-- Written by charter check --fix from db/schema.sql: don't edit. Change the schema, then charter check --fix.

PRAGMA defer_foreign_keys = true;
CREATE TABLE notes_rows AS SELECT * FROM notes;
DROP TABLE notes;
CREATE TABLE notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,                 -- example: 42
  body TEXT NOT NULL,                                   -- example: Buy milk
  author_id TEXT NOT NULL DEFAULT '',                   -- example: 4f1c9a0e7b2d4c6f8a1b3c5d7e9f0a1b; doc: Who wrote it: their id, empty for a program with one of the Worker's own tokens
  author TEXT NOT NULL DEFAULT '',                      -- example: Ada; doc: What they call themselves
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP    -- example: 2026-10-01 12:00:00
);
INSERT INTO notes (id, body, created_at) SELECT id, body, created_at FROM notes_rows;
DROP TABLE notes_rows;
PRAGMA defer_foreign_keys = true;
CREATE TABLE users_rows AS SELECT * FROM users;
DROP TABLE users;
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  role TEXT NOT NULL, -- was: scopes
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO users (id, name, role, created_at) SELECT id, name, scopes, created_at FROM users_rows;
DROP TABLE users_rows;
