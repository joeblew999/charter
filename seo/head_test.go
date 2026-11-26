package seo

import (
	"context"
	"strings"
	"testing"
)

// TestHead: what the audit reads of a page's head, the WebSite structured data only on the home page, and the icon's
// type from its extension.
func TestHead(t *testing.T) {
	render := func(home string) string {
		var b strings.Builder
		if err := Head("Shop & more", "What it sells.", "Shop", "/static/favicon.svg", home, "en").Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	home := render("https://shop.example.com/")
	for _, want := range []string{"<title>Shop &amp; more</title>", `<meta name="description" content="What it sells.">`,
		`<meta property="og:title" content="Shop &amp; more">`, `<meta property="og:site_name" content="Shop">`,
		`<link rel="icon" href="/static/favicon.svg" type="image/svg+xml">`, `"@type":"WebSite"`, `"url":"https://shop.example.com/"`} {
		if !strings.Contains(home, want) {
			t.Errorf("the home page's head has no %s:\n%s", want, home)
		}
	}
	if other := render(""); strings.Contains(other, "application/ld+json") {
		t.Errorf("a page that isn't home has the WebSite structured data:\n%s", other)
	}
}
