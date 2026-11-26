package filters_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/intl/filters"
	"github.com/joeblew999/charter/intl/intltest"
)

func TestMain(m *testing.M) { os.Exit(intltest.Main(m)) }

// TestFilters: each filter is the Intl call it names, in the request's language and zone.
func TestFilters(t *testing.T) {
	intl.Use(intl.Locales{intl.NewLocale("en"), intl.NewLocale("de")}, []bool{true, true})
	ctx := intl.WithRequest(context.Background(), intl.Request{Locale: intl.NewLocale("de"), TimeZone: "Europe/Berlin"})
	at := time.Date(2026, 7, 4, 15, 30, 0, 0, time.UTC)
	for name, tc := range map[string]struct {
		v    intl.Value
		want string
	}{
		"date":           {filters.Date(at), "4.7.2026"},
		"date opts":      {filters.Date(at, "dateStyle", "long", "timeStyle", "short"), "4. Juli 2026 um 17:30"},
		"date pairs":     {filters.Date(at, "dateStyle", "long", "calendar", "hebrew"), "19. Tammus 5786 AM"},
		"number pairs":   {filters.Number("42.195", "style", "unit", "unit", "kilometer", "unitDisplay", "long", "maximumFractionDigits", "1"), "42,2 Kilometer"},
		"dateRange":      {filters.DateRange(at, at.Add(90*time.Minute), "timeStyle", "short"), "17:30–19:00 Uhr"},
		"number int":     {filters.Number(1234567), "1.234.567"},
		"number int64":   {filters.Number(int64(-42)), "-42"},
		"number float":   {filters.Number(0.1), "0,1"},
		"number decimal": {filters.Number("1234.50", "style", "currency", "currency", "EUR"), "1.234,50 €"},
		"duration":       {filters.Duration(26*time.Hour+5*time.Minute, "style", "long"), "1 Tag, 2 Stunden und 5 Minuten"},
		"list":           {filters.List([]string{"Go", "gsx", "htmx"}, "type", "disjunction"), "Go, gsx oder htmx"},
		"zone":           {filters.Zone("Asia/Tokyo"), "Asia/Tokyo · Japanische Normalzeit (GMT+9)"},
		"intl string":    {filters.Intl("JP", "DisplayNames", "of", "type", "region"), "Japan"},
		"intl number":    {filters.Intl(1, "PluralRules", "select"), "one"},
		"intl static":    {filters.Intl("unit", "Intl", "supportedValuesOf"), `["acre",`},
		"intl args":      {filters.Intl(intl.Args{"ä", "z"}, "Collator", "compare"), "-1"},
		"intl range":     {filters.Intl(intl.Args{3, 5}, "NumberFormat", "formatRange", "style", "unit", "unit", "kilogram"), "3–5 kg"},
		"intl no args":   {filters.Intl(intl.Args{}, "PluralRules", "resolvedOptions"), `{"locale":"de"`},
		"since":          {filters.Since(time.Now().Add(-2 * time.Hour)), "vor 2 Stunden"},
	} {
		got := intl.Text(ctx, tc.v)
		if len(got) < len(tc.want) || got[:len(tc.want)] != tc.want {
			t.Errorf("%s = %q, want %q", name, got, tc.want)
		}
	}
	if got := intl.PlainText(ctx, filters.Plain(filters.Number(1234))); got != "1.234" {
		t.Errorf("plain = %q", got)
	}
	if got := filters.Text(ctx, filters.Number(1234)); got != "1.234" {
		t.Errorf("text = %q", got)
	}
}
