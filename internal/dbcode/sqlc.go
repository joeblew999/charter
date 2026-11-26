// Package dbcode is a project's database code, typed, from its schema (db/migrations/*.sql, the one place the tables are
// said). charter check --fix writes it beside the specs (charter.toml's db), and charter check fails when it is stale.
//
//   - Go (db = "sqlc"): the queries of db/queries.sql as functions over charter's cloud package, in db/queries.gen.go. sqlc
//     works out each one's parameters and columns against the schema; charter writes the Go, with no database/sql
//     (charter/d1 says why).
//   - TypeScript (db = "drizzle"): the tables as Drizzle has them, api/tables.ts, read from the schema as SQLite itself
//     has it once the migrations ran; queries are Drizzle's, on drizzle(env.DB).
package dbcode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joeblew999/charter/internal/run"
)

// Column is a column or a parameter as sqlc has it (its codegen request, in JSON).
type Column struct {
	Name        string `json:"name"`
	NotNull     bool   `json:"not_null"`
	IsArray     bool   `json:"is_array"`
	IsSqlcSlice bool   `json:"is_sqlc_slice"`
	Type        struct {
		Name string `json:"name"`
	} `json:"type"`
	Table *struct {
		Name string `json:"name"`
	} `json:"table"`
}

// Query is a query of db/queries.sql as sqlc worked it out.
type Query struct {
	Name     string   `json:"name"`
	Cmd      string   `json:"cmd"`
	Text     string   `json:"text"`
	Comments []string `json:"comments"`
	Columns  []Column `json:"columns"`
	Params   []struct {
		Number int    `json:"number"`
		Column Column `json:"column"`
	} `json:"params"`
}

// Request is what sqlc worked out: the schema's tables and the queries.
type Request struct {
	Catalog struct {
		Schemas []struct {
			Tables []struct {
				Rel struct {
					Name string `json:"name"`
				} `json:"rel"`
				Columns []Column `json:"columns"`
			} `json:"tables"`
		} `json:"schemas"`
	} `json:"catalog"`
	Queries []Query `json:"queries"`
}

// sqlcVersion is the sqlc charter analyzes queries with: a Go program, run with go run, so every system gets it.
const sqlcVersion = "v1.30.0"

// Analyze runs sqlc on dir's schema (db/migrations/) and queries (db/queries.sql), and returns what it worked out. sqlc
// is run with go run, at sqlcVersion.
func Analyze(dir string) (*Request, error) {
	// In the app's own scratch folder (.charter), not the system's: sqlc reads its paths relative to its config, and on
	// Windows the system's temporary folder can be on another drive, which no relative path reaches. And not build/,
	// which a build beside this one (charter dev's) empties.
	work := filepath.Join(dir, ".charter")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(work, "sqlc-")
	if err != nil {
		return nil, err
	}
	if tmp, err = filepath.Abs(tmp); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	rel := func(parts ...string) (string, error) {
		abs, err := filepath.Abs(filepath.Join(append([]string{dir}, parts...)...))
		if err != nil {
			return "", err
		}
		// sqlc reads both relative to its config, even a path that is absolute.
		return filepath.Rel(tmp, abs)
	}
	schema, err := rel("db", "schema.sql")
	if err != nil {
		return nil, err
	}
	queries, err := rel("db", "queries.sql")
	if err != nil {
		return nil, err
	}
	config := fmt.Sprintf("version: \"2\"\nsql:\n  - engine: sqlite\n    schema: %q\n    queries: %q\n    gen:\n      json:\n        out: out\n", filepath.ToSlash(schema), filepath.ToSlash(queries))
	if err := os.WriteFile(filepath.Join(tmp, "sqlc.yaml"), []byte(config), 0o644); err != nil {
		return nil, err
	}
	if _, err := run.Output(dir, map[string]string{"CGO_ENABLED": "0"}, "go", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc@"+sqlcVersion, "generate", "-f", filepath.Join(tmp, "sqlc.yaml")); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(tmp, "out", "codegen_request.json"))
	if err != nil {
		return nil, err
	}
	var r Request
	return &r, json.Unmarshal(b, &r)
}

var initialisms = map[string]bool{"id": true, "url": true, "uri": true, "api": true, "http": true, "json": true, "uuid": true, "sql": true, "ip": true, "html": true, "css": true}

var keywords = map[string]bool{"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true, "defer": true, "else": true,
	"fallthrough": true, "for": true, "func": true, "go": true, "goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true, "switch": true, "type": true, "var": true}

func words(name string) []string {
	return strings.FieldsFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
}

func capital(word string) string {
	if initialisms[strings.ToLower(word)] {
		return strings.ToUpper(word)
	}
	return strings.ToUpper(word[:1]) + word[1:]
}

// exported is a Go name others may use: created_at is CreatedAt, user_id UserID.
func exported(name string) string {
	var b strings.Builder
	for _, w := range words(name) {
		b.WriteString(capital(w))
	}
	return b.String()
}

// local is a Go name of the function's own: created_at is createdAt, id id.
func local(name string) string {
	ws := words(name)
	if len(ws) == 0 {
		ws = []string{"arg"}
	}
	camel := strings.ToLower(ws[0])
	for _, w := range ws[1:] {
		camel += capital(w)
	}
	if keywords[camel] {
		return camel + "_"
	}
	return camel
}

// goType is the Go type of a column, by SQLite's rules of affinity, with dates as the text D1 gives; a column that may
// be NULL is a pointer.
func goType(c Column) (string, error) {
	if c.IsSqlcSlice || c.IsArray {
		return "", fmt.Errorf("db/queries.sql: %s is a list (sqlc.slice): charter writes queries of single values", c.Name)
	}
	name := strings.ToLower(c.Type.Name)
	base := "any"
	switch {
	case strings.Contains(name, "int"):
		base = "int64"
	case regexp.MustCompile(`char|clob|text|date|time`).MatchString(name):
		base = "string"
	case regexp.MustCompile(`real|floa|doub|numeric|decimal`).MatchString(name):
		base = "float64"
	case strings.Contains(name, "bool"):
		base = "bool"
	case strings.Contains(name, "blob"):
		base = "[]byte"
	}
	if c.NotNull || base == "[]byte" || base == "any" {
		return base, nil
	}
	return "*" + base, nil
}

// jsonString is s as JSON writes it (JSON.stringify's quoting, which is valid Go).
func jsonString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// GoQueries is db/queries.gen.go for what sqlc worked out: a function for each query, over charter's cloud package.
func GoQueries(r *Request, pkg string, tags map[string]map[string]string) (string, error) {
	tables := map[string][]Column{}
	for _, s := range r.Catalog.Schemas {
		for _, t := range s.Tables {
			tables[t.Rel.Name] = t.Columns
		}
	}
	var types []string
	made := map[string]bool{}
	// A column's tags: its name in JSON, and what the schema's comment says of it for the API.
	field := func(c Column, table string) (string, error) {
		t, err := goType(c)
		if c.Table != nil {
			table = c.Table.Name
		}
		more := ""
		if tags[table][c.Name] != "" {
			more = " " + tags[table][c.Name]
		}
		return fmt.Sprintf("\t%s %s `json:\"%s\"%s`", exported(c.Name), t, c.Name, more), err
	}
	// The row type of a query: its table's, when it has every column of one table, in order.
	rowOf := func(q Query) (string, error) {
		table := ""
		if len(q.Columns) > 0 && q.Columns[0].Table != nil {
			table = q.Columns[0].Table.Name
		}
		columns, ok := tables[table]
		whole := ok && table != "" && len(columns) == len(q.Columns)
		for i := 0; whole && i < len(q.Columns); i++ {
			whole = q.Columns[i].Table != nil && q.Columns[i].Table.Name == table && q.Columns[i].Name == columns[i].Name
		}
		name, about, fields := q.Name+"Row", "// "+q.Name+"Row is a row of "+q.Name+".", q.Columns
		if whole {
			name, about, fields = exported(table)+"Row", "// "+exported(table)+"Row is a row of the table "+table+".", columns
		}
		if !made[name] {
			made[name] = true
			types = append(types, about, "type "+name+" struct {")
			for _, c := range fields {
				f, err := field(c, table)
				if err != nil {
					return "", err
				}
				types = append(types, f)
			}
			types = append(types, "}", "")
		}
		return name, nil
	}
	var functions []string
	for _, q := range r.Queries {
		if !slices.Contains([]string{":one", ":many", ":exec", ":execrows"}, q.Cmd) {
			return "", fmt.Errorf("db/queries.sql: %s is %s: charter writes :one, :many, :exec and :execrows (the rows a statement changed; for the row itself, RETURNING with :one, where no row is cloud.ErrNoRows)", q.Name, q.Cmd)
		}
		params := slices.Clone(q.Params)
		slices.SortFunc(params, func(a, b struct {
			Number int    `json:"number"`
			Column Column `json:"column"`
		}) int {
			return a.Number - b.Number
		})
		taken := map[string]bool{"db": true}
		signature, args := []string{"db cloud.DB"}, []string{"db", "q" + q.Name}
		for _, p := range params {
			name := local(p.Column.Name)
			for n := 2; taken[name]; n++ {
				name = fmt.Sprintf("%s%d", local(p.Column.Name), n)
			}
			taken[name] = true
			t, err := goType(p.Column)
			if err != nil {
				return "", err
			}
			signature = append(signature, name+" "+t)
			if strings.HasPrefix(t, "*") {
				args = append(args, "cloud.OrNull("+name+")")
			} else {
				args = append(args, name)
			}
		}
		sig, call := strings.Join(signature, ", "), strings.Join(args, ", ")
		// The comment above a query is its doc when it names it, as Go's do; what comes before the first query (the
		// file's own) does not.
		var own []string
		for _, line := range q.Comments {
			own = append(own, strings.TrimSpace(line))
		}
		about := []string{"// " + q.Name + " runs the query " + q.Name + " of db/queries.sql."}
		if len(own) > 0 && strings.HasPrefix(own[0], q.Name+" ") {
			about = nil
			for _, line := range own {
				about = append(about, "// "+line)
			}
		}
		lines := append([]string{"const q" + q.Name + " = " + jsonString(strings.TrimSpace(q.Text)), ""}, about...)
		switch {
		case q.Cmd == ":exec":
			functions = append(functions, strings.Join(append(lines, "func "+q.Name+"("+sig+") error {", "\treturn cloud.Exec("+call+")", "}"), "\n"))
		case q.Cmd == ":execrows":
			functions = append(functions, strings.Join(append(lines, "func "+q.Name+"("+sig+") (int64, error) {", "\treturn cloud.ExecRows("+call+")", "}"), "\n"))
		case len(q.Columns) == 1:
			// One column: its values, not rows of one field.
			c := q.Columns[0]
			t, err := goType(c)
			if err != nil {
				return "", err
			}
			row, err := rowOf(q)
			if err != nil {
				return "", err
			}
			if q.Cmd == ":many" {
				lines = append(lines, "func "+q.Name+"("+sig+") ([]"+t+", error) {", "\trows, err := cloud.Query["+row+"]("+call+")",
					"\tvalues := make([]"+t+", len(rows))", "\tfor i, row := range rows {", "\t\tvalues[i] = row."+exported(c.Name), "\t}", "\treturn values, err", "}")
			} else {
				lines = append(lines, "func "+q.Name+"("+sig+") ("+t+", error) {", "\trow, err := cloud.One["+row+"]("+call+")", "\treturn row."+exported(c.Name)+", err", "}")
			}
			functions = append(functions, strings.Join(lines, "\n"))
		case len(q.Columns) == 0:
			return "", fmt.Errorf("db/queries.sql: %s is %s but returns no columns: make it :exec, or add RETURNING", q.Name, q.Cmd)
		default:
			row, err := rowOf(q)
			if err != nil {
				return "", err
			}
			result, fn := row, "One"
			if q.Cmd == ":many" {
				result, fn = "[]"+row, "Query"
			}
			functions = append(functions, strings.Join(append(lines, "func "+q.Name+"("+sig+") ("+result+", error) {", "\treturn cloud."+fn+"["+row+"]("+call+")", "}"), "\n"))
		}
	}
	source := strings.Join(append(append([]string{
		"// Code generated by charter check --fix from db/schema.sql and db/queries.sql, with sqlc. DO NOT EDIT.",
		"// Change those, then charter check --fix.",
		"",
		"package " + pkg,
		"",
		`import "github.com/joeblew999/charter/cloud"`,
		"",
	}, types...), strings.Join(functions, "\n\n"), ""), "\n")
	out, err := format.Source([]byte(source))
	if err != nil {
		return "", fmt.Errorf("the Go written for db/queries.sql does not format: %w\n%s", err, source)
	}
	return string(out), nil
}

// Write writes dir's database code (db = "sqlc") from its schema; with check it writes nothing, and fails if what is
// there is not what the schema gives.
func Write(dir, db string, check bool, loss ...bool) error {
	if db != "sqlc" {
		return fmt.Errorf(`charter.toml: db = %q: "sqlc"`, db)
	}
	// The schema first: the migration for what changed in db/schema.sql, then the queries typed against it.
	if _, err := Migrate(dir, check, len(loss) > 0 && loss[0]); err != nil {
		return err
	}
	return writeQueries(dir, check)
}

// writeQueries writes dir's db/queries.gen.go (package db) from its schema and queries; with check it writes nothing, and
// fails if the one there is not what they give.
func writeQueries(dir string, check bool) error {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return errors.New("db/queries.sql is written as Go: there is no go.mod here")
	}
	pkg := "db"
	r, err := Analyze(dir)
	if err != nil {
		return err
	}
	schema, _ := os.ReadFile(filepath.Join(dir, "db", "schema.sql"))
	if !check {
		for _, note := range partRows(r) {
			fmt.Println("· " + note)
		}
	}
	text, err := GoQueries(r, pkg, Tags(string(schema)))
	if err != nil {
		return err
	}
	file := filepath.Join(dir, "db", "queries.gen.go")
	if !check {
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			return err
		}
		fmt.Println("db/queries.gen.go: from db/schema.sql and db/queries.sql (sqlc)")
		return nil
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != text {
		return errors.New("db/queries.gen.go is not what db/schema.sql and db/queries.sql give: charter check --fix")
	}
	fmt.Println("db/queries.gen.go is what db/schema.sql gives")
	return nil
}

// partRows says which queries return some of a table's columns, by name: their rows are a type of their own, not the
// table's row, which is what a developer who has just added a column to the schema trips over (the Go that used the
// table's row no longer compiles, and says nothing of the query).
func partRows(r *Request) (notes []string) {
	tables := map[string]int{}
	for _, s := range r.Catalog.Schemas {
		for _, t := range s.Tables {
			tables[t.Rel.Name] = len(t.Columns)
		}
	}
	for _, q := range r.Queries {
		if len(q.Columns) < 2 || q.Columns[0].Table == nil {
			continue
		}
		table, one := q.Columns[0].Table.Name, true
		for _, c := range q.Columns {
			one = one && c.Table != nil && c.Table.Name == table
		}
		if one && len(q.Columns) < tables[table] {
			notes = append(notes, fmt.Sprintf("db/queries.sql: %s returns %d of the %s table's %d columns, so its rows are %sRow, not %sRow. To return the whole row, new columns too: SELECT * or RETURNING *",
				q.Name, len(q.Columns), table, tables[table], q.Name, exported(table)))
		}
	}
	return notes
}
