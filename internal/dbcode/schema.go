package dbcode

import (
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // SQLite in Go, as D1 is SQLite: for working the migrations out, in the command only
)

// The database's schema is db/schema.sql: the tables as they are now, in one file. The migrations (db/migrations)
// are written from it: charter check --fix works out what the migrations so far make, what the schema says, and
// writes the next migration for the difference; charter check fails while there is one. Nobody writes a migration.
//
// What a difference becomes:
//
//   - a new table or index: its CREATE; one that is gone: its DROP;
//   - columns added at the end of a table, each one SQLite can add in place: ALTER TABLE … ADD COLUMN;
//   - anything else about a table (a column's type, a constraint, a column gone or moved): the table made again
//     under the new definition, the rows copied over, the old one dropped, as SQLite's own guide has it.
//
// A column that was renamed says so in the schema, on its line: `title TEXT NOT NULL, -- was: name`. Without it a
// rename is a column dropped and another added: a change that loses data is refused, until it is asked for
// (charter check --fix --drop). Views and triggers are dropped and made again when their text changes.

// header marks a migration charter wrote.
const header = "-- Written by charter check --fix from db/schema.sql: don't edit. Change the schema, then charter check --fix.\n"

// other is a view or a trigger: something with no rows of its own, dropped and made again when its text changes.
type other struct{ kind, sql string }

type table struct {
	name    string
	sql     string   // its CREATE TABLE, as SQLite keeps it
	columns []string // in order
	defs    []string // each column's definition, and after them the table's constraints
	indexes map[string]string
}

// state is the tables a list of statements makes.
func state(statements string) (map[string]*table, error) {
	tables, _, err := stateAll(statements)
	return tables, err
}

func stateAll(statements string) (map[string]*table, map[string]other, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(statements); err != nil {
		return nil, nil, err
	}
	tables, others := map[string]*table{}, map[string]other{}
	rows, err := db.Query(`SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' AND name NOT LIKE 'd1_%' AND name NOT LIKE '_cf_%' ORDER BY type DESC, name`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, name, on, text string
		if err := rows.Scan(&kind, &name, &on, &text); err != nil {
			return nil, nil, err
		}
		switch kind {
		case "table":
			tables[name] = &table{name: name, sql: text, defs: definitions(text), indexes: map[string]string{}}
		case "index":
			if t := tables[on]; t != nil && text != "" {
				t.indexes[name] = text
			}
		case "view", "trigger":
			others[name] = other{kind, text}
		}
	}
	for _, t := range tables {
		cols, err := db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, t.name)
		if err != nil {
			return nil, nil, err
		}
		for cols.Next() {
			var name string
			cols.Scan(&name)
			t.columns = append(t.columns, name)
		}
		cols.Close()
	}
	return tables, others, rows.Err()
}

var (
	comment    = regexp.MustCompile(`--[^\n]*`)
	spaces     = regexp.MustCompile(`\s+`)
	constraint = regexp.MustCompile(`(?i)^(PRIMARY\s+KEY|UNIQUE|CHECK|FOREIGN\s+KEY|CONSTRAINT)\b`)
)

// definitions are what is between a CREATE TABLE's brackets, split at its commas: the column definitions, then the
// table's constraints, each without comments and with single spaces.
func definitions(create string) []string {
	text := comment.ReplaceAllString(create, "")
	open, shut := strings.Index(text, "("), strings.LastIndex(text, ")")
	if open < 0 || shut < open {
		return nil
	}
	var parts []string
	depth, start, quote := 0, open+1, byte(0)
	for i := open + 1; i < shut; i++ {
		switch c := text[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			parts = append(parts, text[start:i])
			start = i + 1
		}
	}
	parts = append(parts, text[start:shut])
	for i, p := range parts {
		parts[i] = strings.TrimSpace(spaces.ReplaceAllString(p, " "))
	}
	return parts
}

// addable says whether SQLite can add a column of this definition to a table that has rows.
func addable(def string) bool {
	upper := strings.ToUpper(def)
	if strings.Contains(upper, "PRIMARY KEY") || strings.Contains(upper, "UNIQUE") || strings.Contains(upper, "REFERENCES") {
		return false
	}
	if strings.Contains(upper, "NOT NULL") && !strings.Contains(upper, "DEFAULT") {
		return false
	}
	return !regexp.MustCompile(`(?i)DEFAULT\s*(\(|CURRENT_)`).MatchString(def)
}

// renames are a schema's `-- was: old` notes: table, then the column's name now, to its name before.
func renames(schema string) map[string]map[string]string {
	out := map[string]map[string]string{}
	current := ""
	for _, line := range strings.Split(schema, "\n") {
		if m := regexp.MustCompile(`(?i)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["\x60]?(\w+)`).FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if m := regexp.MustCompile(`^\s*["\x60]?(\w+)["\x60]?\s.*--.*\bwas:\s*(\w+)`).FindStringSubmatch(line); m != nil && current != "" {
			if out[current] == nil {
				out[current] = map[string]string{}
			}
			out[current][m[1]] = m[2]
		}
	}
	return out
}

// Diff is the statements that take a database from what `from` makes to what `to` makes, what they are called
// together (for the migration's name), and what a person should know of them.
func Diff(from, to string) (statements []string, name string, notes []string, err error) {
	before, beforeOthers, err := stateAll(from)
	if err != nil {
		return nil, "", nil, fmt.Errorf("db/migrations: %w", err)
	}
	after, afterOthers, err := stateAll(to)
	if err != nil {
		return nil, "", nil, fmt.Errorf("db/schema.sql: %w", err)
	}
	// Views and triggers that are gone or changed go first: a table they read may be made again below.
	same := func(a, b string) bool {
		return spaces.ReplaceAllString(comment.ReplaceAllString(a, ""), " ") == spaces.ReplaceAllString(comment.ReplaceAllString(b, ""), " ")
	}
	var again []string
	for _, n := range slices.Sorted(maps.Keys(beforeOthers)) {
		if now, ok := afterOthers[n]; !ok || !same(now.sql, beforeOthers[n].sql) {
			statements = append(statements, "DROP "+strings.ToUpper(beforeOthers[n].kind)+" "+n+";")
		}
	}
	for _, n := range slices.Sorted(maps.Keys(afterOthers)) {
		if was, ok := beforeOthers[n]; !ok || !same(was.sql, afterOthers[n].sql) {
			again = append(again, afterOthers[n].sql+";")
		}
	}
	defer func() {
		// A table made again takes every view and trigger down with it for a moment: they all go first and come back
		// after, whether their own text changed or not.
		if err == nil && slices.ContainsFunc(statements, func(st string) bool {
			return strings.HasPrefix(st, "CREATE TABLE ") && strings.Contains(st, "_rows AS SELECT * FROM ")
		}) {
			var drops []string
			again = again[:0]
			for _, n := range slices.Sorted(maps.Keys(beforeOthers)) {
				drops = append(drops, "DROP "+strings.ToUpper(beforeOthers[n].kind)+" "+n+";")
			}
			for _, n := range slices.Sorted(maps.Keys(afterOthers)) {
				again = append(again, afterOthers[n].sql+";")
			}
			statements = append(drops, slices.DeleteFunc(statements, func(st string) bool { return slices.Contains(drops, st) })...)
		}
		if err == nil && len(again) > 0 {
			statements = append(statements, again...)
			if name == "" {
				name = "views_and_triggers"
			}
		}
	}()
	was := renames(to)
	var words []string
	names := func(m map[string]*table) []string {
		out := make([]string, 0, len(m))
		for n := range m {
			out = append(out, n)
		}
		slices.Sort(out)
		return out
	}
	indexes := func(t *table) (out []string) {
		for _, n := range slices.Sorted(func(yield func(string) bool) {
			for n := range t.indexes {
				if !yield(n) {
					return
				}
			}
		}) {
			out = append(out, t.indexes[n]+";")
		}
		return out
	}
	for _, n := range names(after) {
		t, old := after[n], before[n]
		switch {
		case old == nil:
			statements = append(statements, t.sql+";")
			statements = append(statements, indexes(t)...)
			words = append(words, "create_"+n)
		case slices.Equal(old.defs, t.defs):
			// The table is as it was: only its indexes may differ.
			for _, i := range slices.Sorted(func(yield func(string) bool) {
				for i := range old.indexes {
					if !yield(i) {
						return
					}
				}
			}) {
				if spaces.ReplaceAllString(t.indexes[i], " ") != spaces.ReplaceAllString(old.indexes[i], " ") {
					statements = append(statements, "DROP INDEX "+i+";")
					words = append(words, n+"_indexes")
				}
			}
			for i, text := range t.indexes {
				if spaces.ReplaceAllString(old.indexes[i], " ") != spaces.ReplaceAllString(text, " ") {
					statements = append(statements, text+";")
					words = append(words, n+"_indexes")
				}
			}
		default:
			// Columns added at the end, each addable in place, and nothing else: ALTER TABLE.
			oldCols, newCols := len(old.columns), len(t.columns)
			inPlace := newCols > oldCols && len(t.defs)-newCols == len(old.defs)-oldCols &&
				slices.Equal(old.defs[:oldCols], t.defs[:oldCols]) && slices.Equal(old.defs[oldCols:], t.defs[newCols:])
			for i := oldCols; inPlace && i < newCols; i++ {
				inPlace = addable(t.defs[i]) && !constraint.MatchString(t.defs[i])
			}
			if inPlace {
				for i := oldCols; i < newCols; i++ {
					statements = append(statements, "ALTER TABLE "+n+" ADD COLUMN "+t.defs[i]+";")
					words = append(words, n+"_add_"+t.columns[i])
				}
				continue
			}
			// Anything else: the table again, its rows copied.
			var into, out []string
			for i, c := range t.columns {
				source := c
				if w := was[n][c]; w != "" && slices.Contains(old.columns, w) {
					source = w
				}
				if !slices.Contains(old.columns, source) {
					upper := strings.ToUpper(t.defs[i])
					if strings.Contains(upper, "NOT NULL") && !strings.Contains(upper, "DEFAULT") {
						return nil, "", nil, fmt.Errorf("db/schema.sql: %s.%s is NOT NULL without a DEFAULT, and the table's rows have no value for it: give it a DEFAULT, or say which column it was (-- was: old_name)", n, c)
					}
					continue
				}
				into, out = append(into, c), append(out, source)
			}
			for _, c := range old.columns {
				if !slices.Contains(out, c) {
					notes = append(notes, fmt.Sprintf("%s.%s is dropped, with what its rows held (a column that was renamed says so in the schema: -- was: %s)", n, c, c))
				}
			}
			// The rows are set aside, the table dropped and made as it now is, and the rows put back under its own
			// name. The order matters where other tables' rows point at this one (a foreign key): D1 checks those when
			// the migration ends, and they are only good again once this table, by this name, has its rows back. A
			// new table that was merely given the name afterwards would leave every such row pointing at nothing.
			aside := n + "_rows"
			statements = append(statements,
				"PRAGMA defer_foreign_keys = true;",
				fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM %s;", aside, n),
				"DROP TABLE "+n+";",
				t.sql+";",
				fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s;", n, strings.Join(into, ", "), strings.Join(out, ", "), aside),
				"DROP TABLE "+aside+";")
			statements = append(statements, indexes(t)...)
			words = append(words, "change_"+n)
		}
	}
	for _, n := range names(before) {
		if after[n] == nil {
			statements = append(statements, "DROP TABLE "+n+";")
			notes = append(notes, fmt.Sprintf("the table %s is dropped, with its rows", n))
			words = append(words, "drop_"+n)
		}
	}
	words = slices.Compact(words)
	if len(words) > 3 {
		words = append(words[:3], "and_more")
	}
	return statements, strings.Join(words, "_"), notes, nil
}

// Migrate keeps dir's db/migrations in step with its db/schema.sql. With check it writes nothing, and fails when the
// schema has a change no migration makes. Otherwise it writes the migration for the change: one file for everything
// not yet committed, written again as the schema moves on, so that a change worked on in several steps is one
// migration when it is committed.
func Migrate(dir string, check, loss bool) (written string, err error) {
	schema, err := os.ReadFile(filepath.Join(dir, "db", "schema.sql"))
	if err != nil {
		return "", nil // an app whose migrations are its own
	}
	files, _ := filepath.Glob(filepath.Join(dir, "db", "migrations", "*.sql"))
	slices.Sort(files)
	var done strings.Builder
	pending := ""
	for i, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		// The last migration, written by charter and not yet in git: it is the one being worked on.
		if !check && i == len(files)-1 && strings.HasPrefix(string(b), header) && untracked(dir, f) {
			pending = f
			continue
		}
		done.Write(b)
		done.WriteString("\n")
	}
	statements, name, notes, err := Diff(done.String(), string(schema))
	if err != nil {
		return "", err
	}
	if check {
		if len(statements) > 0 {
			return "", errors.New("db/schema.sql has changes no migration in db/migrations makes: charter check --fix writes it")
		}
		return "", nil
	}
	if len(notes) > 0 && !loss {
		return "", fmt.Errorf("db/schema.sql would lose data:\n  · %s\nA column that was renamed says so on its line (-- was: old_name). To drop it for good: charter check --fix --drop", strings.Join(notes, "\n  · "))
	}
	if len(statements) == 0 {
		if pending != "" {
			return "", os.Remove(pending) // the schema went back to what the migrations make
		}
		return "", nil
	}
	number := len(files) + 1
	if pending != "" {
		number = len(files)
		if err := os.Remove(pending); err != nil {
			return "", err
		}
	}
	text := header
	for _, n := range notes {
		text += "-- Note: " + n + ".\n"
	}
	text += "\n" + strings.Join(statements, "\n") + "\n"
	file := filepath.Join(dir, "db", "migrations", fmt.Sprintf("%04d_%s.sql", number, name))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return "", err
	}
	for _, n := range notes {
		fmt.Println("· " + n)
	}
	rel, _ := filepath.Rel(dir, file)
	fmt.Println(rel + ": from db/schema.sql")
	return file, os.WriteFile(file, []byte(text), 0o644)
}

// untracked says git does not have the file yet (or there is no git here).
func untracked(dir, file string) bool {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", file)
	cmd.Dir = dir
	return cmd.Run() != nil
}

// Tags are what a schema's comments say of its columns for the API (-- example: 42; doc: The note's id), by table
// and column: they become the row struct's tags, so that a row is also what the contract serves.
func Tags(schema string) map[string]map[string]string {
	out := map[string]map[string]string{}
	current := ""
	table := regexp.MustCompile(`(?i)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["\x60]?(\w+)`)
	column := regexp.MustCompile(`^\s*["\x60]?(\w+)["\x60]?\s[^-]*--\s*(.*)$`)
	for _, line := range strings.Split(schema, "\n") {
		if m := table.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		m := column.FindStringSubmatch(line)
		if m == nil || current == "" {
			continue
		}
		var tags []string
		for _, part := range strings.Split(m[2], ";") {
			key, value, ok := strings.Cut(strings.TrimSpace(part), ":")
			if value = strings.Trim(strings.TrimSpace(value), `"`); ok && (key == "example" || key == "doc") && value != "" {
				tags = append(tags, fmt.Sprintf("%s:%q", key, value))
			}
		}
		if len(tags) > 0 {
			if out[current] == nil {
				out[current] = map[string]string{}
			}
			out[current][m[1]] = strings.Join(tags, " ")
		}
	}
	return out
}

// Table is a table of a schema, as SQLite itself has it once the schema ran: for what describes a database to a
// reader (the docs' page of it).
type Table struct {
	Name    string
	Columns []TableColumn
}

// TableColumn is a column: its type as declared, whether it may be NULL, its default as SQL ("" for none), whether
// it is the primary key or part of it, and what the schema's comment says of it (doc: …).
type TableColumn struct {
	Name, Type, Default, Doc string
	NotNull, Key             bool
}

// Tables are the tables a schema makes, in the order it declares them, read from SQLite after running it: so they
// are what the database has, not what a reading of the text takes it to say.
func Tables(schema string) ([]Table, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'd1_%' AND name NOT LIKE '_cf_%' ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		names = append(names, name)
	}
	rows.Close()
	docs := Tags(schema)
	said := regexp.MustCompile(`doc:"((?:[^"\\]|\\.)*)"`)
	var tables []Table
	for _, name := range names {
		t := Table{Name: name}
		cols, err := db.Query(`SELECT name, type, "notnull", COALESCE(dflt_value, ''), pk FROM pragma_table_info(?) ORDER BY cid`, name)
		if err != nil {
			return nil, err
		}
		for cols.Next() {
			var c TableColumn
			var notNull, key int
			if err := cols.Scan(&c.Name, &c.Type, &notNull, &c.Default, &key); err != nil {
				cols.Close()
				return nil, err
			}
			c.NotNull, c.Key = notNull != 0 || key != 0, key != 0
			if m := said.FindStringSubmatch(docs[name][c.Name]); m != nil {
				c.Doc, _ = strconv.Unquote(`"` + m[1] + `"`)
			}
			t.Columns = append(t.Columns, c)
		}
		cols.Close()
		tables = append(tables, t)
	}
	return tables, nil
}
