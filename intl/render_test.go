package intl_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/intl"
)

func render(t *testing.T, ctx context.Context, v intl.Value) string {
	t.Helper()
	var b strings.Builder
	if err := intl.Render(ctx, v).Render(ctx, &b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestValue: a Value renders the Intl result in the request's language and zone, inside the element the browser redoes.
func TestValue(t *testing.T) {
	intl.Use(intl.Locales{intl.NewLocale("en"), intl.NewLocale("de"), intl.NewLocale("ar")}, []bool{true, true, true})
	de := intl.WithRequest(context.Background(), intl.Request{Locale: intl.NewLocale("de"), TimeZone: "Europe/Berlin"})
	at := time.Date(2026, 7, 4, 15, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		v    intl.Value
		want string
	}{
		{intl.Date(at, intl.Options("dateStyle", "full", "timeStyle", "short")),
			`<time datetime="2026-07-04T15:30:00Z" data-intl="DateTimeFormat" data-opts="{&#34;dateStyle&#34;:&#34;full&#34;,&#34;timeStyle&#34;:&#34;short&#34;}" data-args="[1783179000000]">Samstag, 4. Juli 2026 um 17:30</time>`},
		{intl.Number("1234.5", intl.Options("style", "currency", "currency", "EUR")), ">1.234,50\u00a0€</data>"},
		{intl.Int(1234567), `<data value="1234567" data-intl="NumberFormat" data-args="[&#34;1234567&#34;]">1.234.567</data>`},
		{intl.List([]string{"Go", "htmx", "gsx"}, ""), `>Go, htmx und gsx</span>`},
		{intl.Duration(26*time.Hour+5*time.Minute, intl.Options("style", "long")), `>1 Tag, 2 Stunden und 5 Minuten</data>`},
		{intl.Duration(400*time.Millisecond, ""), `>0 Sek.</data>`},
		{intl.Duration(0, intl.Options("style", "long")), `>0 Sekunden</data>`},
		{intl.Of("DisplayNames", "of", intl.Options("type", "region"), intl.JSON("JP")), `>Japan</span>`},
		{intl.Of("PluralRules", "select", intl.Options("type", "ordinal"), "2"), `>other</span>`},
		{intl.Of("Intl", "getCanonicalLocales", "", intl.JSON("EN-us")), `>[&#34;en-US&#34;]</span>`},
		{intl.Zone("Asia/Tokyo"), `data-intl="zone" data-args="[&#34;Asia/Tokyo&#34;]">Asia/Tokyo · Japanische Normalzeit (GMT+9)</span>`},
	} {
		if got := render(t, de, tc.v); !strings.Contains(got, tc.want) {
			t.Errorf("%+v:\n got %s\nwant …%s", tc.v.Call, got, tc.want)
		}
	}
	if got := render(t, de, intl.Since(time.Now().Add(-3*time.Minute))); !strings.Contains(got, `data-intl="since"`) || !strings.HasSuffix(got, `>vor 3 Minuten</time>`) {
		t.Errorf("Since: %s", got)
	}
	ar := intl.NewLocale("ar")
	if ar.Dir() != "rtl" || ar.Name() != "العربية" || ar.Maximal() != "ar-Arab-EG" {
		t.Errorf("ar facts: %q %q %q", ar.Dir(), ar.Name(), ar.Maximal())
	}
}
