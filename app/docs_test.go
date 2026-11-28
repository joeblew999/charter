package app

import (
	"strings"
	"testing"
)

// The list at the side has the pages of one group together under its name, in the order the docs are read; and
// each page knows the one before it and the one after, with none at either end.
func TestDocsGroupsAndAround(t *testing.T) {
	d := &Docs{Pages: []Doc{
		{Slug: "", Title: "Home"}, {Slug: "start", Title: "Starting", Group: "Start"}, {Slug: "layout", Title: "Layout", Group: "Start"},
		{Slug: "pages", Title: "Pages", Group: "Write"}, {Slug: "other", Title: "Other"},
	}}
	var got []string
	for _, g := range d.Groups() {
		var slugs []string
		for _, p := range g.Pages {
			slugs = append(slugs, p.Slug)
		}
		got = append(got, g.Name+"["+strings.Join(slugs, " ")+"]")
	}
	if want := "[] Start[start layout] Write[pages] [other]"; strings.Join(got, " ") != want {
		t.Errorf("groups %q, want %q", strings.Join(got, " "), want)
	}
	if before, after := d.Around(d.Pages[0]); before != nil || after == nil || after.Slug != "start" {
		t.Errorf("around the first page: %v, %v", before, after)
	}
	if before, after := d.Around(d.Pages[2]); before == nil || before.Slug != "start" || after == nil || after.Slug != "pages" {
		t.Errorf("around a page in the middle: %v, %v", before, after)
	}
	if before, after := d.Around(d.Pages[4]); before == nil || before.Slug != "pages" || after != nil {
		t.Errorf("around the last page: %v, %v", before, after)
	}
}
