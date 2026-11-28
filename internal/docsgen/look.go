package docsgen

import (
	"bytes"
	"html"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// How a docs page looks is gsxui's, as its own site's docs are (https://ui.gsxhq.dev/docs): nothing here or in the
// docs screen says a colour, a size or a rule of its own.
//
//   - A page is a container marked data-doc, which gsxui's own stylesheet for its docs styles (headings, paragraphs,
//     links, code: web/site.css in gsxui, written into the app as web/gsxui-site/site.css by internal/themes), and
//     a section for each heading of the second level, as gsxui's own doc pages are laid out.
//   - What that stylesheet leaves to a page's markup has the classes gsxui's own doc pages give it: a list, a
//     heading that is linked to. They are Tailwind's, written here once.
//   - A table is data (its head and its rows), which the docs screen shows with gsxui's Table component, in the
//     reader's style.
//   - Code is marked by what each word is under the names gsxui's site gives them (ts-keyword, ts-string: its
//     highlighter's, which is tree-sitter and not Go), so gsxui's stylesheet colours it.
//
// The docs screen has these classes in its own text too, so that Tailwind, which reads the app's templates, has
// them: a test holds the two together (app/screens).
const (
	ListClass    = "list-disc space-y-2 ps-6"
	OrderedClass = "list-decimal space-y-2 ps-6"
	HeadingClass = "scroll-mt-20"
	// A link has what gsxui's stylesheet gives one in a paragraph, wherever it is: in a list, in a table.
	LinkClass    = "font-medium underline decoration-foreground/30 hover:decoration-foreground"
	FigureClass  = "flex flex-col gap-1"
	CaptionClass = "font-mono text-xs text-muted-foreground"
)

// Classes are every class above, for the test that holds the docs screen to them.
var Classes = []string{ListClass, OrderedClass, HeadingClass, LinkClass, FigureClass, CaptionClass}

// A page is its sections, and a section its parts: a run of HTML, or a table.
type section struct {
	Parts []part
}

type part struct {
	HTML string     `json:",omitempty"`
	Head []string   `json:",omitempty"` // a table's head: each cell's HTML
	Rows [][]string `json:",omitempty"`
}

// sectioned is a page as the docs screen shows it: a section from each heading of the second level to the next
// (what is above the first is one too), each a run of HTML and the tables in it.
func sectioned(md goldmark.Markdown, source []byte, doc ast.Node) ([]section, error) {
	var out []section
	var now section
	var run bytes.Buffer
	flush := func() {
		if run.Len() > 0 {
			now.Parts = append(now.Parts, part{HTML: run.String()})
			run.Reset()
		}
	}
	inside := func(n ast.Node) (string, error) {
		var b bytes.Buffer
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if err := md.Renderer().Render(&b, source, c); err != nil {
				return "", err
			}
		}
		return b.String(), nil
	}
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok && h.Level == 2 {
			flush()
			if len(now.Parts) > 0 {
				out = append(out, now)
			}
			now = section{}
		}
		table, ok := n.(*east.Table)
		if !ok {
			if err := md.Renderer().Render(&run, source, n); err != nil {
				return nil, err
			}
			continue
		}
		flush()
		p := part{Head: []string{}, Rows: [][]string{}}
		for row := table.FirstChild(); row != nil; row = row.NextSibling() {
			var cells []string
			for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
				text, err := inside(cell)
				if err != nil {
					return nil, err
				}
				cells = append(cells, text)
			}
			if _, head := row.(*east.TableHeader); head {
				p.Head = cells
			} else {
				p.Rows = append(p.Rows, cells)
			}
		}
		now.Parts = append(now.Parts, p)
	}
	flush()
	if len(now.Parts) > 0 {
		out = append(out, now)
	}
	return out, nil
}

// code renders a sample of code: each word in a span named for what it is, as gsxui's site names them.
type code struct{}

func (code) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindFencedCodeBlock, func(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		n := node.(*ast.FencedCodeBlock)
		var text bytes.Buffer
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			text.Write(line.Value(source))
		}
		_, _ = w.WriteString("<pre><code>")
		lexer := lexers.Get(string(n.Language(source)))
		if lexer == nil {
			_, _ = w.WriteString(html.EscapeString(text.String()))
		} else {
			tokens, err := chroma.Coalesce(lexer).Tokenise(nil, text.String())
			if err != nil {
				return ast.WalkStop, err
			}
			for token := tokens(); token != chroma.EOF; token = tokens() {
				if class := Named(token.Type); class != "" {
					_, _ = w.WriteString(`<span class="` + class + `">` + html.EscapeString(token.Value) + `</span>`)
				} else {
					_, _ = w.WriteString(html.EscapeString(token.Value))
				}
			}
		}
		_, _ = w.WriteString("</code></pre>\n")
		return ast.WalkSkipChildren, nil
	})
}

// Names are the names gsxui's site gives the words of a sample that this gives too. A test holds each to gsxui's
// own stylesheet: one gsxui no longer colours fails there.
var Names = []string{"ts-keyword", "ts-type", "ts-type-builtin", "ts-constant-builtin", "ts-function", "ts-function-builtin", "ts-tag", "ts-attribute", "ts-string", "ts-string-escape", "ts-number", "ts-comment", "ts-operator", "ts-punctuation-delimiter"}

// Named is the name gsxui's site gives a word of this kind, or "" for one it leaves as it is.
func Named(t chroma.TokenType) string {
	switch {
	case t == chroma.KeywordType:
		return "ts-type-builtin"
	case t == chroma.KeywordConstant || t == chroma.NameConstant:
		return "ts-constant-builtin"
	case t.InCategory(chroma.Keyword):
		return "ts-keyword"
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic:
		return "ts-function"
	case t == chroma.NameBuiltin || t == chroma.NameBuiltinPseudo:
		return "ts-function-builtin"
	case t == chroma.NameClass:
		return "ts-type"
	case t == chroma.NameTag:
		return "ts-tag"
	case t == chroma.NameAttribute:
		return "ts-attribute"
	case t == chroma.LiteralStringEscape:
		return "ts-string-escape"
	case t.InSubCategory(chroma.LiteralString):
		return "ts-string"
	case t.InSubCategory(chroma.LiteralNumber):
		return "ts-number"
	case t.InCategory(chroma.Comment):
		return "ts-comment"
	case t.InCategory(chroma.Operator):
		return "ts-operator"
	case t == chroma.Punctuation:
		return "ts-punctuation-delimiter"
	}
	return ""
}
