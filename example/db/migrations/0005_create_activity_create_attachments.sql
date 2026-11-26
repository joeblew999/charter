-- Written by charter check --fix from db/schema.sql: don't edit. Change the schema, then charter check --fix.

CREATE TABLE activity (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL,
  note_id INTEGER NOT NULL DEFAULT 0,
  author TEXT NOT NULL DEFAULT '',
  notes INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
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
