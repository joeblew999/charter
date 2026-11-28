package dbcode

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Shape is what each query takes and gives, as sqlc worked it out: "PersonByToken :one (hash TEXT!) id TEXT!, …". Two
// analyses of the same queries with the same shapes are the same Go.
func Shape(r *Request) map[string]string {
	column := func(c Column) string {
		s := c.Name + " " + strings.ToUpper(c.Type.Name)
		if c.NotNull {
			s += "!"
		}
		return s
	}
	shapes := map[string]string{}
	for _, q := range r.Queries {
		params := slices.Clone(q.Params)
		slices.SortFunc(params, func(a, b struct {
			Number int    `json:"number"`
			Column Column `json:"column"`
		}) int {
			return a.Number - b.Number
		})
		var takes, gives []string
		for _, p := range params {
			takes = append(takes, column(p.Column))
		}
		for _, c := range q.Columns {
			gives = append(gives, column(c))
		}
		shapes[q.Name] = fmt.Sprintf("%s (%s) %s", q.Cmd, strings.Join(takes, ", "), strings.Join(gives, ", "))
	}
	return shapes
}

// Fits fails when an app's db/schema.sql no longer has tables as queries need them: queries are charter's own, on
// tables it wrote into the app's schema (what), and the app owns its copy. They are run through sqlc against the
// app's schema, as the app's own queries are, and must come out as they do against charter's (schema): a column
// that is gone, renamed or of another type is said here, and not by a sign-in that fails on a Worker.
func Fits(dir, what, schema, queries string) error {
	work := filepath.Join(dir, ".charter")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(work, "fits-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	theirs, ours := filepath.Join(tmp, "queries.sql"), filepath.Join(tmp, "schema.sql")
	if err := os.WriteFile(theirs, []byte(queries), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(ours, []byte(schema), 0o644); err != nil {
		return err
	}
	want, err := AnalyzeFiles(dir, ours, theirs)
	if err != nil {
		return err
	}
	got, err := AnalyzeFiles(dir, filepath.Join(dir, "db", "schema.sql"), theirs)
	if err != nil {
		return fmt.Errorf("db/schema.sql no longer has the tables of %s as charter reads and writes them:\n%w\ncharter's are:\n%s", what, err, schema)
	}
	wanted, has := Shape(want), Shape(got)
	var differ []string
	for name, shape := range wanted {
		if has[name] != shape {
			differ = append(differ, fmt.Sprintf("  %s: charter's statement is %s, and on this schema it is %s", name, shape, has[name]))
		}
	}
	if len(differ) > 0 {
		slices.Sort(differ)
		return fmt.Errorf("db/schema.sql has the tables of %s with other types than charter reads and writes:\n%s\ncharter's are:\n%s", what, strings.Join(differ, "\n"), schema)
	}
	return nil
}

// cloudPath is the package whose statements take SQL as text.
const cloudPath = "github.com/joeblew999/charter/cloud"

// byHand are cloud's functions that take a statement as text. Generated code calls them with a statement sqlc has
// checked; anything else that does has written SQL nothing checks. cloud.Batch is not among them: it takes
// statements, which the generated …Stmt functions make. A cloud.Statement written out (cloud.Statement{Query: …})
// is one by hand too; the type's name alone (a []cloud.Statement for a batch) is not.
var byHand = []string{"Query", "One", "Exec", "ExecRows", "Stmt"}

// aroundCloud are packages that reach D1 around cloud: workers-go's database/sql driver, which calls the binding
// itself. A statement through it is in no request's session (it reads the primary always and gives its reader no
// bookmark: cloud/session.go) and is checked against no schema.
var aroundCloud = []string{"github.com/syumai/workers-go/cloudflare/d1"}

// HandWritten is every place in dir's Go, outside generated files, that gives cloud a statement as text, or
// reaches D1 around cloud: "api/store.go:41: cloud.Exec", "api/store.go:7: imports …/cloudflare/d1". A statement belongs in db/queries.sql, where sqlc checks its columns and types
// against db/schema.sql and charter writes it as a function. allowed are files that may (paths relative to dir).
func HandWritten(dir string, allowed ...string) ([]string, error) {
	var found []string
	files := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "build" || name == "dist" || name == "testdata") {
				return filepath.SkipDir
			}
			// Another module (an app inside charter's repository) is checked as itself.
			if path != dir {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".gen.go") || slices.Contains(allowed, rel) {
			return nil
		}
		file, err := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil // what does not parse is the compiler's to say
		}
		local := ""
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if slices.Contains(aroundCloud, p) {
				found = append(found, fmt.Sprintf("%s:%d: imports %s", rel, files.Position(imp.Pos()).Line, p))
			}
			if p == cloudPath {
				local = "cloud"
				if imp.Name != nil {
					local = imp.Name.Name
				}
			}
		}
		if local == "" || local == "_" {
			return nil
		}
		of := func(e ast.Expr) string {
			if sel, ok := e.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == local {
					return sel.Sel.Name
				}
			}
			return ""
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CompositeLit:
				if of(n.Type) == "Statement" {
					found = append(found, fmt.Sprintf("%s:%d: cloud.Statement{…}", rel, files.Position(n.Pos()).Line))
				}
			case *ast.SelectorExpr:
				if slices.Contains(byHand, of(n)) {
					found = append(found, fmt.Sprintf("%s:%d: cloud.%s", rel, files.Position(n.Pos()).Line, n.Sel.Name))
				}
			}
			return true
		})
		return nil
	})
	return found, err
}
