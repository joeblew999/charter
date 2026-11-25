package intl_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/intl/intltest"
)

func TestMain(m *testing.M) { os.Exit(intltest.Main(m)) }

// TestInt: counts in the language's digits and grouping (en-IN groups in lakhs, Spanish leaves four digits alone).
func TestInt(t *testing.T) {
	for _, tc := range []struct {
		locale string
		v      int64
		want   string
	}{{"en", 1234567, "1,234,567"}, {"de", -1234, "-1.234"}, {"en-IN", 1234567, "12,34,567"}, {"es", 1234, "1234"}} {
		ctx := intl.WithRequest(context.Background(), intl.Request{Locale: intl.NewLocale(tc.locale)})
		if got := intl.Text(ctx, intl.Int(tc.v)); got != tc.want {
			t.Errorf("Int(%s, %d) = %q, want %q", tc.locale, tc.v, got, tc.want)
		}
	}
}

// TestJSOptions: options shown as JavaScript writes them.
func TestJSOptions(t *testing.T) {
	for want, opts := range map[string]string{
		"{}":                                     "",
		`{dateStyle: "full", timeStyle: "long"}`: intl.Options("dateStyle", "full", "timeStyle", "long"),
		`{maximumFractionDigits: 0}`:             intl.Options("maximumFractionDigits", "0"),
		`{hour: "numeric", timeZone: "America/New_York", hour12: true}`: intl.Options("hour", "numeric", "timeZone", "America/New_York", "hour12", "true"),
		`{type: "a, \"b\": c"}`: intl.Options("type", `a, "b": c`),
	} {
		if got := (intl.Call{Opts: opts}).JSOptions(); got != want {
			t.Errorf("JSOptions(%s) = %s, want %s", opts, got, want)
		}
	}
}

// TestClock: the hour-cycle labels, in a language whose own clock is 12-hour, whatever clock the viewer chose.
func TestClock(t *testing.T) {
	ctx := intl.WithRequest(context.Background(), intl.Request{Locale: intl.NewLocale("en"), HourCycle: "h23"})
	for hc, want := range map[string]string{"": "1:05 PM", "h12": "1:05 PM", "h23": "13:05"} {
		if got := intl.Text(ctx, intl.Clock(hc)); got != want {
			t.Errorf("Clock(%q) = %q, want %q", hc, got, want)
		}
	}
}

// TestCalendar: a month calendar's words come from Intl, per language; a month inside a date takes the form a date
// needs (Russian "января", not the caption's "январь").
func TestCalendar(t *testing.T) {
	intl.Use(intl.Locales{intl.NewLocale("en"), intl.NewLocale("de"), intl.NewLocale("ar"), intl.NewLocale("ru"), intl.NewLocale("ja"), intl.NewLocale("ar-EG")}, []bool{true, true, true, true, true, true})
	in := func(id string) context.Context {
		return intl.WithRequest(context.Background(), intl.Request{Locale: intl.NewLocale(id)})
	}
	for id, want := range map[string]struct {
		month, inDate, weekday, caption, day, digits string
		first                                        time.Weekday
	}{
		"en":    {"January", "January", "Sunday", "{month} {year}", "{weekday}, {month} {day}, {year}", "", time.Sunday},
		"de":    {"Januar", "Januar", "Sonntag", "{month} {year}", "{weekday}, {day}. {month} {year}", "", time.Monday},
		"ru":    {"январь", "января", "воскресенье", "{month} {year} г.", "{weekday}, {day} {month} {year} г.", "", time.Monday},
		"ja":    {"1", "1", "日曜日", "{year}年{month}月", "{year}年{month}月{day}日{weekday}", "", time.Sunday},
		"ar":    {"يناير", "يناير", "الأحد", "{month} {year}", "{weekday}، {day} {month} {year}", "", time.Saturday},
		"ar-EG": {"يناير", "يناير", "الأحد", "{month} {year}", "{weekday}، {day} {month} {year}", "٠١٢٣٤٥٦٧٨٩", time.Saturday},
	} {
		c := intl.CalendarOf(in(id))
		if c.Months[0] != want.month || c.MonthsFormat[0] != want.inDate || c.Weekdays[0] != want.weekday || c.Caption != want.caption ||
			c.DayLabel != want.day || c.Digits != want.digits {
			t.Errorf("%s: %q %q %q %q %q %q", id, c.Months[0], c.MonthsFormat[0], c.Weekdays[0], c.Caption, c.DayLabel, c.Digits)
		}
		if got := intl.WeekStart(in(id)); got != want.first {
			t.Errorf("%s: week starts %v, want %v", id, got, want.first)
		}
	}
}

// TestSegmentsOrder: text split and sorted the language's way.
func TestSegmentsOrder(t *testing.T) {
	intl.Use(intl.Locales{intl.NewLocale("en"), intl.NewLocale("de"), intl.NewLocale("ja"), intl.NewLocale("sv")}, []bool{true, true, true, true})
	in := func(id string) context.Context {
		return intl.WithRequest(context.Background(), intl.Request{Locale: intl.NewLocale(id)})
	}
	if got := intl.Segments(in("en"), "Zoë 👨‍👩‍👧!", "grapheme"); len(got) != 6 || got[4] != "👨‍👩‍👧" {
		t.Errorf("graphemes = %q", got)
	}
	if got := intl.Segments(in("ja"), "今日は良い天気です。", "word"); len(got) < 4 {
		t.Errorf("Japanese words = %q", got)
	}
	if got := intl.Segments(in("en"), "Smith arrived. He sat down! ", "sentence"); len(got) != 2 {
		t.Errorf("sentences = %q", got)
	}
	names := []string{"Zürich", "Österreich", "Oslo", "Ägypten", "Zypern"}
	sorted := func(id string) string {
		var out []string
		for _, i := range intl.Order(in(id), names, "") {
			out = append(out, names[i])
		}
		return strings.Join(out, " ")
	}
	if got := sorted("de"); got != "Ägypten Oslo Österreich Zürich Zypern" {
		t.Errorf("German order: %s", got)
	}
	if got := sorted("sv"); got != "Oslo Zypern Zürich Ägypten Österreich" { // Swedish: ü sorts as y, Ä and Ö after Z
		t.Errorf("Swedish order: %s", got)
	}
}
