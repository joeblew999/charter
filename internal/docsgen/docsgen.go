// Package docsgen turns an app's docs (docs/*.md) into data the app carries: docs/docs.built.json, each page as HTML
// and as it was written, with the few lines of Go that name it (docs.Pages). It runs in charter build, so the
// Worker carries the pages and no Markdown code.
//
// A page's place is its file's name without the number that orders it: docs/index.md is /docs/, docs/03-pages.md is
// /docs/pages. Its title is its first heading, its description its first paragraph. A link to another page is
// written to its file ([the pages](03-pages.md)) and becomes a link to its address, which works on GitHub too.
package docsgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

var ordered = regexp.MustCompile(`^\d+[-_]`)

// slug is a docs file's place under /docs.
func slug(file string) string {
	name := ordered.ReplaceAllString(strings.TrimSuffix(filepath.Base(file), ".md"), "")
	if name == "index" {
		return ""
	}
	return name
}

type page struct {
	Slug, Title, Description, HTML, Markdown, Source, Group string
	Headings                                                [][3]string // id, title, depth
}

// Write writes dir's docs/docs.built.go from its docs/*.md, and reports how many pages; none when there is no docs
// folder.
func Write(dir string) (int, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "docs", "*.md"))
	if len(files) == 0 {
		return 0, nil
	}
	// index.md first, then the files' own order.
	slices.SortFunc(files, func(a, b string) int {
		if (slug(a) == "") != (slug(b) == "") {
			if slug(a) == "" {
				return -1
			}
			return 1
		}
		return strings.Compare(a, b)
	})
	// Code is marked up by what each word is (chroma's classes, no colours): the docs screen gives them the app's
	// theme's colours, in light and in dark.
	md := goldmark.New(goldmark.WithExtensions(extension.GFM, highlighting.NewHighlighting(highlighting.WithFormatOptions(chromahtml.WithClasses(true)))),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()), goldmark.WithRendererOptions(html.WithUnsafe()))
	// What a person wrote (docs/*.md), then what charter writes of the app itself (reference.go).
	type source struct {
		slug, from string
		written    []byte
	}
	var sources []source
	for _, file := range files {
		written, err := os.ReadFile(file)
		if err != nil {
			return 0, err
		}
		sources = append(sources, source{slug(file), filepath.ToSlash(filepath.Join("docs", filepath.Base(file))), written})
	}
	generated, err := reference(dir)
	if err != nil {
		return 0, err
	}
	for _, g := range generated {
		if slices.ContainsFunc(sources, func(s source) bool { return s.slug == g.slug }) {
			return 0, fmt.Errorf("docs/ has a page at %s, which is where charter writes the reference for %s: give the file another name", g.slug, g.from)
		}
		sources = append(sources, source{g.slug, g.from, []byte(g.markdown)})
	}
	// The groups the pages are listed in, and their order, are the front page's: docs/index.md (grouped, below).
	group, order := map[string]string{}, map[string]int{}
	if len(sources) > 0 && sources[0].slug == "" {
		group, order = grouped(md, sources[0].written)
	}
	for _, g := range generated {
		if _, listed := group[g.slug]; !listed {
			group[g.slug] = "Reference"
		}
	}
	// index.md, then the pages it lists in its order, then the rest as they were (a file's number, then what
	// charter writes).
	rank := func(s source, i int) int {
		switch at, listed := order[s.slug]; {
		case s.slug == "":
			return -1
		case listed:
			return at
		}
		return len(order) + i
	}
	at := map[string]int{}
	for i, s := range sources {
		at[s.slug] = rank(s, i)
	}
	slices.SortStableFunc(sources, func(a, b source) int { return at[a.slug] - at[b.slug] })
	var pages []page
	for _, src := range sources {
		written := src.written
		// gsx is read as Go, which it mostly is; a sample taken from a file says which.
		source := []byte(strings.ReplaceAll(captioned(string(written)), "```gsx\n", "```go\n"))
		p := page{Slug: src.slug, Source: src.from, Group: group[src.slug]}
		doc := md.Parser().Parse(text.NewReader(source))
		ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch n := n.(type) {
			case *ast.Heading:
				title := plain(n, source)
				if n.Level == 1 && p.Title == "" {
					p.Title = title
				}
				if id, ok := n.AttributeString("id"); ok && (n.Level == 2 || n.Level == 3) {
					p.Headings = append(p.Headings, [3]string{string(id.([]byte)), title, fmt.Sprint(n.Level)})
				}
			case *ast.Paragraph:
				if p.Description == "" && n.Parent() == doc {
					p.Description = short(plain(n, source))
				}
			case *ast.Link:
				// A link to another page's file is a link to its address, from this page's.
				if dest := string(n.Destination); strings.HasSuffix(strings.SplitN(dest, "#", 2)[0], ".md") && !strings.Contains(dest, "://") {
					target, anchor, _ := strings.Cut(dest, "#")
					to := slug(target)
					if to == "" {
						to = "./"
					}
					if anchor != "" {
						to += "#" + anchor
					}
					n.Destination = []byte(to)
				}
			}
			return ast.WalkContinue, nil
		})
		if p.Title == "" {
			return 0, fmt.Errorf("%s has no heading (# Title): a page is named by its first", p.Source)
		}
		var out bytes.Buffer
		if err := md.Renderer().Render(&out, source, doc); err != nil {
			return 0, fmt.Errorf("%s: %w", p.Source, err)
		}
		p.HTML, p.Markdown = out.String(), string(written)
		pages = append(pages, p)
	}
	// The pages are data (docs.built.json), which the Go only names: on a Worker charter build puts it beside the Wasm,
	// so a page that is edited needs no build of the Go at all; anywhere else (a test) the package embeds the file.
	type heading struct {
		ID, Title string
		Depth     int
	}
	type doc struct {
		Slug, Title, Description, HTML, Markdown, Source, Group string
		Headings                                                []heading
	}
	docs := make([]doc, len(pages))
	for i, p := range pages {
		docs[i] = doc{Slug: p.Slug, Title: p.Title, Description: p.Description, HTML: p.HTML, Markdown: p.Markdown, Source: p.Source, Group: p.Group, Headings: []heading{}}
		for _, h := range p.Headings {
			depth, _ := strconv.Atoi(h[2])
			docs[i].Headings = append(docs[i].Headings, heading{h[0], h[1], depth})
		}
	}
	data, err := json.MarshalIndent(docs, "", "\t")
	if err != nil {
		return 0, err
	}
	header := "// Code generated by charter build from docs/*.md. DO NOT EDIT: change the Markdown.\n\n"
	for name, code := range map[string]string{
		"docs.built.json": string(data) + "\n",
		"docs.built.go": header + "package docs\n\nimport \"github.com/joeblew999/charter/app\"\n\n" +
			"// Pages are the docs, index.md first, then in the files' order: each Markdown file of this folder as a page\n// (docs.built.json).\nvar Pages = app.BuiltDocs(built)\n",
		"docs_native.built.go": "//go:build !(js && wasm)\n\n" + header + "package docs\n\nimport _ \"embed\"\n\n" +
			"// built is the pages, for whatever is no Worker: a test, the program that writes the API's specs.\n//\n//go:embed docs.built.json\nvar built string\n",
		"docs_worker.built.go": "//go:build js && wasm\n\n" + header + "package docs\n\n" +
			"// built is empty on a Worker: the pages are beside the Wasm (build/docs.mjs), where charter build put them.\nvar built string\n",
	} {
		file := filepath.Join(dir, "docs", name)
		if old, err := os.ReadFile(file); err == nil && string(old) == code {
			continue
		}
		if err := os.WriteFile(file, []byte(code), 0o644); err != nil {
			return 0, err
		}
	}
	return len(pages), nil
}

// grouped reads the groups of an app's docs from their front page, where a reader finds them too: a line that is
// only bold (**Start**) names a group, and the pages the list under it links to (their files: 01-start.md) are in
// it, in that order. So the list at the side of every page is the front page's own, said once. A front page with
// no such lines groups nothing, and the pages are listed as their files are numbered.
func grouped(md goldmark.Markdown, index []byte) (group map[string]string, order map[string]int) {
	group, order = map[string]string{}, map[string]int{}
	doc := md.Parser().Parse(text.NewReader(index))
	current := ""
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Paragraph:
			current = ""
			if strong, ok := n.FirstChild().(*ast.Emphasis); ok && strong.Level == 2 && n.ChildCount() == 1 {
				current = plain(n, index)
			}
		case *ast.List:
			if current == "" {
				continue
			}
			ast.Walk(n, func(l ast.Node, entering bool) (ast.WalkStatus, error) {
				if link, ok := l.(*ast.Link); ok && entering {
					file, _, _ := strings.Cut(string(link.Destination), "#")
					if strings.HasSuffix(file, ".md") && !strings.Contains(file, "://") {
						if _, seen := order[slug(file)]; !seen && slug(file) != "" {
							group[slug(file)], order[slug(file)] = current, len(order)
						}
					}
				}
				return ast.WalkContinue, nil
			})
		default:
			current = ""
		}
	}
	return group, order
}

// short is a paragraph as a search result has room for: its text, cut at a word within 155 characters.
func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= 155 {
		return s
	}
	r := []rune(s)[:155]
	cut := strings.LastIndex(string(r), " ")
	return strings.TrimRight(string(r)[:cut], ",;:") + "…"
}

// plain is a node's text as a reader sees it: a link's words without its address, code without its marks.
func plain(n ast.Node, source []byte) string {
	var b strings.Builder
	ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch c := c.(type) {
			case *ast.Text:
				b.Write(c.Segment.Value(source))
				if c.SoftLineBreak() || c.HardLineBreak() {
					b.WriteByte(' ')
				}
			case *ast.String:
				b.Write(c.Value)
			}
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}
