package docsgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A code sample in the docs that is taken from a real file says so on its fence:
//
//	```go file=../example/pages/pages.go#Home.Props
//
// file is a path from the app's folder; after # is the declaration wanted, by its name (a type, a function, a method
// as Type.Method, a constant, a gsx component), with its comment. Without #, it is the whole file. The Markdown
// carries the code too, so it reads on GitHub; Samples holds it to the file: a sample that no longer says what the
// code says is a failure of charter check, and check --fix writes it again.

var sampleFence = regexp.MustCompile("(?ms)^```(\\w+)[ \\t]+file=(\\S+)[ \\t]*\\n(.*?)^```[ \\t]*$")

// Samples compares every sample of dir's docs/*.md that names a file with that file, and with fix writes the
// Markdown again where they differ. It returns the samples that differ (and, with fix, were written).
func Samples(dir string, fix bool) ([]string, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "docs", "*.md"))
	var differ []string
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var failed error
		out := sampleFence.ReplaceAllStringFunc(string(b), func(block string) string {
			m := sampleFence.FindStringSubmatch(block)
			lang, from, body := m[1], m[2], m[3]
			want, err := sample(dir, from)
			if err != nil {
				failed = fmt.Errorf("docs/%s: %w", filepath.Base(file), err)
				return block
			}
			if strings.TrimSpace(body) == strings.TrimSpace(want) {
				return block
			}
			differ = append(differ, "docs/"+filepath.Base(file)+": "+from)
			return "```" + lang + " file=" + from + "\n" + strings.TrimRight(want, "\n") + "\n```"
		})
		if failed != nil {
			return nil, failed
		}
		if fix && out != string(b) {
			if err := os.WriteFile(file, []byte(out), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return differ, nil
}

// sample is what from names: a file, or one declaration of it.
func sample(dir, from string) (string, error) {
	path, name, _ := strings.Cut(from, "#")
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
	if err != nil {
		return "", fmt.Errorf("the sample's file: %w", err)
	}
	switch {
	case name == "":
		return string(b), nil
	case strings.HasSuffix(path, ".go"):
		return goDecl(path, b, name)
	case strings.HasSuffix(path, ".gsx"):
		return gsxDecl(path, string(b), name)
	}
	return "", fmt.Errorf("%s: a declaration can be named in a .go or a .gsx file only", from)
}

// goDecl is the declaration called name in a Go file, with its comment: Type, Func, Type.Method, a constant.
func goDecl(path string, src []byte, name string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return "", err
	}
	recv, method, isMethod := strings.Cut(name, ".")
	for _, d := range f.Decls {
		var doc *ast.CommentGroup
		found := false
		switch d := d.(type) {
		case *ast.FuncDecl:
			doc = d.Doc
			if isMethod {
				found = d.Recv != nil && len(d.Recv.List) == 1 && d.Name.Name == method && typeName(d.Recv.List[0].Type) == recv
			} else {
				found = d.Recv == nil && d.Name.Name == name
			}
		case *ast.GenDecl:
			doc = d.Doc
			for _, spec := range d.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					found = found || spec.Name.Name == name
				case *ast.ValueSpec:
					for _, n := range spec.Names {
						found = found || n.Name == name
					}
				}
			}
		}
		if !found {
			continue
		}
		start := d.Pos()
		if doc != nil {
			start = doc.Pos()
		}
		return string(src[fset.Position(start).Offset:fset.Position(d.End()).Offset]), nil
	}
	return "", fmt.Errorf("%s has no declaration called %s", path, name)
}

func typeName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return typeName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return ""
}

// gsxDecl is the component, function or type called name in a gsx file, with its comment: it starts at the start
// of a line and ends at the first closing brace that does.
func gsxDecl(path, src, name string) (string, error) {
	recv, method, isMethod := strings.Cut(name, ".")
	pattern := `(?m)^(?:component|func|type) ` + regexp.QuoteMeta(name) + `\b`
	if isMethod {
		pattern = `(?m)^(?:component|func) \(\w+ \*?` + regexp.QuoteMeta(recv) + `\) ` + regexp.QuoteMeta(method) + `\b`
	}
	loc := regexp.MustCompile(pattern).FindStringIndex(src)
	if loc == nil {
		return "", fmt.Errorf("%s has no declaration called %s", path, name)
	}
	start := loc[0]
	// The comment above it: the lines before that start with //.
	for start > 0 {
		prev := strings.LastIndex(src[:start-1], "\n") + 1
		if !strings.HasPrefix(src[prev:start], "//") {
			break
		}
		start = prev
	}
	end := strings.Index(src[loc[0]:], "\n}\n")
	if end < 0 {
		return "", fmt.Errorf("%s: %s does not end with a closing brace at the start of a line", path, name)
	}
	return src[start : loc[0]+end+2], nil
}

// captioned is a page's Markdown as it is rendered: a sample that names its file says which, above the code.
func captioned(markdown string) string {
	return sampleFence.ReplaceAllStringFunc(markdown, func(block string) string {
		m := sampleFence.FindStringSubmatch(block)
		path, _, _ := strings.Cut(m[2], "#")
		for strings.HasPrefix(path, "../") {
			path = strings.TrimPrefix(path, "../")
		}
		return "<figure class=\"" + FigureClass + "\"><figcaption class=\"" + CaptionClass + "\">" + path + "</figcaption>\n\n```" + m[1] + "\n" + m[3] + "```\n\n</figure>"
	})
}
