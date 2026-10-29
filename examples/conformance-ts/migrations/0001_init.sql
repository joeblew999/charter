CREATE TABLE IF NOT EXISTS notes (seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE, body TEXT NOT NULL);
INSERT OR IGNORE INTO notes (id, body) VALUES ('n1', 'note n1'), ('n2', 'note n2'), ('n3', 'note n3'), ('n4', 'note n4'), ('n5', 'note n5');
