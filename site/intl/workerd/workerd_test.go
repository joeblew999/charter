package workerd_test

import (
	"context"
	"testing"

	"github.com/joeblew999/charter/site/intl"
	"github.com/joeblew999/charter/site/intl/workerd"
)

// TestStart: what an ordinary Go server gets: intl formats through workerd's Intl, the repo's intl daemon.
func TestStart(t *testing.T) {
	shipped := intl.Locales{intl.NewLocale("en"), intl.NewLocale("de")}
	intl.Use(shipped, []bool{true, true})
	if err := workerd.Start(); err != nil {
		t.Fatal(err)
	}
	de := intl.WithRequest(context.Background(), intl.Request{Locale: shipped[1], TimeZone: "Europe/Berlin"})
	if got := intl.Text(de, intl.Int(1234567)); got != "1.234.567" {
		t.Errorf("Int in German = %q, want 1.234.567", got)
	}
}
