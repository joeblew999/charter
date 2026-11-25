package seo_test

import (
	"testing"

	"github.com/joeblew999/charter/seo"
)

func TestSnippetWidth(t *testing.T) {
	for s, want := range map[string]int{"": 0, "Home": 4, "Café": 4, "ホーム": 6, "服务器，": 8, "gsx 组件": 8, "مكونات": 6} {
		if got := seo.SnippetWidth(s); got != want {
			t.Errorf("SnippetWidth(%q) = %d, want %d", s, got, want)
		}
	}
}
