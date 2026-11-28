// Package scopecode holds an app to declaring each scope once.
//
// A scope is a value (app.Scope), so that a word cannot be written where one is needed. NewScope is what makes one
// from a word, and it is for a declaration: a variable of a package, which everything else names.
//
//	var Write = charter.NewScope("write")
//
// Made anywhere else (Needs: charter.NewScope("wrtie")) it is the word written by hand again, under another
// spelling of the same mistake. Lint finds those.
package scopecode

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

// The packages that have NewScope.
var packages = []string{"github.com/joeblew999/charter/api", "github.com/joeblew999/charter/app"}

// Lint is every place in dir's Go, outside generated files, that makes a scope where it is not being declared:
// "pages/pages.go:132: charter.NewScope". A declaration is a package's variable whose value the call is.
func Lint(dir string) ([]string, error) {
	var found []string
	files := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "build" || name == "dist" || name == "testdata" || name == "sdk") {
				return filepath.SkipDir
			}
			if path != dir {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".gen.go") || strings.HasSuffix(name, ".built.go") {
			return nil
		}
		file, err := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil // what does not parse is the compiler's to say
		}
		var local []string
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if !slices.Contains(packages, p) {
				continue
			}
			as := p[strings.LastIndex(p, "/")+1:]
			if imp.Name != nil {
				as = imp.Name.Name
			}
			local = append(local, as)
		}
		if len(local) == 0 {
			return nil
		}
		// The calls that are a declaration: the values of the package's own variables.
		declared := map[ast.Node]bool{}
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				for _, spec := range gen.Specs {
					for _, value := range spec.(*ast.ValueSpec).Values {
						declared[value] = true
					}
				}
			}
		}
		rel, _ := filepath.Rel(dir, path)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || declared[call] {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewScope" {
				if pkg, ok := sel.X.(*ast.Ident); ok && slices.Contains(local, pkg.Name) {
					found = append(found, fmt.Sprintf("%s:%d: %s.NewScope", filepath.ToSlash(rel), files.Position(call.Pos()).Line, pkg.Name))
				}
			}
			return true
		})
		return nil
	})
	slices.Sort(found)
	return found, err
}
