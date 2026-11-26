// The gsx side of intl: Render, the renderer
// gsx.toml names for Value, so { t |> date(…) } and { Date(…) } render in the request's language; Attrs and
// HTML, for spreading into an element; and Head, the <head> of a page's languages. An app without gsx uses intl alone:
// Text, and Value.Element for its own templates.
package intl

import (
	"context"

	"github.com/gsxhq/gsx"
)

// Render is Value's gsx renderer (gsx.toml [renderers], the gsx Package renderers pattern): the result in the
// request's language, time zone and clock, in the element Value.Element says, which js/intl-browser.mjs redoes in
// the viewer's own zone.
func Render(ctx context.Context, v Value) gsx.Node {
	tag, attrs, text := v.Element(ctx)
	a := gsxAttrs(attrs)
	switch tag {
	case "time":
		return <time { a... }>{ text }</time>
	case "data":
		return <data { a... }>{ text }</data>
	}
	return <span { a... }>{ text }</span>
}

// Attrs is v's call as js/intl-browser.mjs reads it, for an element that can't hold Render's (an <option>): spread it
// there with v.Plain() as the text, <option { Attrs(v)... }>{ v.Plain() }</option>.
func Attrs(v Value) gsx.Attrs { return gsxAttrs(v.Attrs()) }

// HTML is the <html> element's language attributes for the request (HTML): <html { HTML(ctx)... }>.
func HTML(ctx context.Context) gsx.Attrs { return gsxAttrs(htmlAttrs(ctx)) }

func gsxAttrs(attrs []Attr) gsx.Attrs {
	a := make(gsx.Attrs, len(attrs))
	for i, at := range attrs {
		a[i] = gsx.Attr{Key: at.Key, Value: at.Value}
	}
	return a
}
