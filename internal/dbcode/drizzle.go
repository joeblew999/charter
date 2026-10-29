package dbcode

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	_ "modernc.org/sqlite" // SQLite in pure Go, as D1 is SQLite
)

// column is a column as SQLite itself has it (PRAGMA table_info).
type column struct {
	name, typ string
	notNull   bool
	dflt      sql.NullString
	pk        int
}

var (
	textType   = regexp.MustCompile(`char|clob|text|date|time`)
	realType   = regexp.MustCompile(`real|floa|doub`)
	identifier = regexp.MustCompile(`^[A-Za-z_$][\w$]*$`)
	autoIncr   = regexp.MustCompile(`(?i)autoincrement`)
)

// DrizzleTables is api/tables.ts: each table the migrations make, as Drizzle has it.
func DrizzleTables(migrations []string) (string, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return "", err
	}
	defer db.Close()
	// One connection: each has its own database in memory.
	db.SetMaxOpenConns(1)
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return "", fmt.Errorf("a migration: %w", err)
		}
	}
	type table struct{ name, sql string }
	var tables []table
	rows, err := db.Query(`SELECT name, sql FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite\_%' ESCAPE '\' AND name NOT LIKE '\_cf\_%' ESCAPE '\' AND name <> 'd1_migrations' ORDER BY name`)
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var t table
		if err := rows.Scan(&t.name, &t.sql); err != nil {
			rows.Close()
			return "", err
		}
		tables = append(tables, t)
	}
	rows.Close()
	builders := map[string]bool{"sqliteTable": true}
	defaults := false
	var blocks []string
	for _, t := range tables {
		columns, err := columnsOf(db, t.name)
		if err != nil {
			return "", err
		}
		unique, err := uniqueColumns(db, t.name)
		if err != nil {
			return "", err
		}
		var keys []column
		for _, c := range columns {
			if c.pk > 0 {
				keys = append(keys, c)
			}
		}
		slices.SortFunc(keys, func(a, b column) int { return a.pk - b.pk })
		lines := []string{"/** The table " + t.name + ". */", "export const " + tableName(t.name) + " = sqliteTable(" + jsonString(t.name) + ", {"}
		for _, c := range columns {
			builder := builderOf(strings.ToLower(c.typ))
			builders[builder] = true
			field := builder + "(" + jsonString(c.name) + ")"
			switch {
			case len(keys) == 1 && c.pk > 0 && autoIncr.MatchString(t.sql):
				field += ".primaryKey({ autoIncrement: true })"
			case len(keys) == 1 && c.pk > 0:
				field += ".primaryKey()"
			case c.notNull:
				field += ".notNull()"
			}
			if unique[c.name] {
				field += ".unique()"
			}
			if c.dflt.Valid {
				defaults = true
				field += ".default(sql`" + strings.NewReplacer(`\`, `\\`, "`", "\\`", "${", `\${`).Replace(c.dflt.String) + "`)"
			}
			key := c.name
			if !identifier.MatchString(key) {
				key = jsonString(key)
			}
			lines = append(lines, "\t"+key+": "+field+",")
		}
		end := "});"
		if len(keys) > 1 {
			builders["primaryKey"] = true
			var cols []string
			for _, k := range keys {
				cols = append(cols, "table["+jsonString(k.name)+"]")
			}
			end = "}, table => [primaryKey({ columns: [" + strings.Join(cols, ", ") + "] })]);"
		}
		blocks = append(blocks, strings.Join(append(lines, end), "\n"))
	}
	out := []string{
		"// Written by `charter spec` from migrations/*.sql: don't edit. Change the schema with a new",
		"// migration, then charter spec. The tables as Drizzle has them, for typed queries on D1:",
		"//   drizzle(env.DB).select().from(table)",
	}
	if defaults {
		out = append(out, `import { sql } from "drizzle-orm";`)
	}
	var names []string
	for b := range builders {
		names = append(names, b)
	}
	slices.Sort(names)
	out = append(out, "import { "+strings.Join(names, ", ")+` } from "drizzle-orm/sqlite-core";`, "", strings.Join(blocks, "\n\n"), "")
	return strings.Join(out, "\n"), nil
}

// builderOf is Drizzle's column builder for a column's declared type, by SQLite's rules of affinity.
func builderOf(typ string) string {
	switch {
	case strings.Contains(typ, "int"):
		return "integer"
	case textType.MatchString(typ):
		return "text"
	case realType.MatchString(typ):
		return "real"
	case strings.Contains(typ, "blob") || typ == "":
		return "blob"
	}
	return "numeric"
}

// tableName is a table's name in TypeScript: note_tags is noteTags.
func tableName(name string) string {
	var b strings.Builder
	for i, w := range words(name) {
		if i == 0 {
			b.WriteString(strings.ToLower(w))
		} else {
			b.WriteString(capital(w))
		}
	}
	return b.String()
}

func columnsOf(db *sql.DB, table string) ([]column, error) {
	rows, err := db.Query("PRAGMA table_info(" + jsonString(table) + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []column
	for rows.Next() {
		var c column
		var cid, notNull int
		if err := rows.Scan(&cid, &c.name, &c.typ, &notNull, &c.dflt, &c.pk); err != nil {
			return nil, err
		}
		c.notNull = notNull != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

// uniqueColumns is each column of table a UNIQUE constraint holds on its own.
func uniqueColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query("PRAGMA index_list(" + jsonString(table) + ")")
	if err != nil {
		return nil, err
	}
	var indexes []string
	for rows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			rows.Close()
			return nil, err
		}
		if unique != 0 && origin == "u" {
			indexes = append(indexes, name)
		}
	}
	rows.Close()
	out := map[string]bool{}
	for _, index := range indexes {
		rows, err := db.Query("PRAGMA index_info(" + jsonString(index) + ")")
		if err != nil {
			return nil, err
		}
		var cols []string
		for rows.Next() {
			var seqno, cid int
			var name string
			if err := rows.Scan(&seqno, &cid, &name); err != nil {
				rows.Close()
				return nil, err
			}
			cols = append(cols, name)
		}
		rows.Close()
		if len(cols) == 1 {
			out[cols[0]] = true
		}
	}
	return out, nil
}

// migrations are dir's migrations/*.sql, in the order they are applied.
func migrations(dir string) ([]string, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "migrations", "*.sql"))
	slices.Sort(files)
	var out []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		out = append(out, string(b))
	}
	return out, nil
}

// writeTables writes dir's api/tables.ts from its schema; with check it writes nothing, and fails if the one there is
// not what the schema gives.
func writeTables(dir string, check bool) error {
	ms, err := migrations(dir)
	if err != nil {
		return err
	}
	text, err := DrizzleTables(ms)
	if err != nil {
		return err
	}
	file := filepath.Join(dir, "api", "tables.ts")
	if !check {
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			return err
		}
		fmt.Println("api/tables.ts: from migrations/")
		return nil
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != text {
		return errors.New("api/tables.ts is not what migrations/ gives: charter spec")
	}
	fmt.Println("api/tables.ts is what migrations/ gives")
	return nil
}
