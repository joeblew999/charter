-- created_at is never NULL (its default sets it): the schema says so, so the typed code npm run spec
-- writes has it as text, as the API does. SQLite changes a column only by making the table again; the
-- ids stay, and as no note is ever deleted, the next one follows the newest.
CREATE TABLE notes_new (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
INSERT INTO notes_new (id, body, created_at) SELECT id, body, COALESCE(created_at, CURRENT_TIMESTAMP) FROM notes;
DROP TABLE notes;
ALTER TABLE notes_new RENAME TO notes;
