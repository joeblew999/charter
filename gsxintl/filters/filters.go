// Package filters is intl as gsx pipeline filters. Register the package in gsx.toml and every function here is a
// filter, its name with the first letter lowercased (gsx Configuration guide, filter_packages):
//
//	filter_packages = ["github.com/joeblew999/charter/gsxintl/filters"]
//
//	{ note.Created |> since }                              vor 3 Minuten
//	{ order.Placed |> date("dateStyle", "long") }          4. Juli 2026
//	{ total |> number("style", "currency", "currency", c) }
//	{ "JP" |> intl("DisplayNames", "of", "type", "region") }
//	{ intl.Args{"ä", "z"} |> intl("Collator", "compare") }  several arguments
//	title={ v |> plain }                               a Value's text, for an attribute
//	title={ locales.BoardTitle |> text }               a message's text, for a component's string input
//	href={ "/about" |> localize }                      an in-app path in the request's locale: /de/about
//	href={ pages.About{} |> url }                      a page's address in the request's locale: /de/about
//
// Each returns an intl.Value (rendered by gsxintl.Render, in the request's language and redone by the browser in the
// viewer's zone), except plain and text. Options are Intl's own option names and values, in pairs (intl.Options);
// none for Intl's defaults.
package filters

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/joeblew999/charter/intl"
)

// Date is t with Intl.DateTimeFormat: { t |> date } or { t |> date("dateStyle", "full", "timeStyle", "short") }.
func Date(t time.Time, opts ...string) intl.Value { return intl.Date(t, first(opts)) }

// DateRange is t to end with Intl.DateTimeFormat's formatRange: { start |> dateRange(end) }.
func DateRange(t, end time.Time, opts ...string) intl.Value {
	return intl.DateRange(t, end, first(opts))
}

// Since is t relative to now with Intl.RelativeTimeFormat, kept fresh in the browser: { t |> since }.
func Since(t time.Time) intl.Value { return intl.Since(t) }

// Number is n with Intl.NumberFormat: an integer, a float, or a decimal string (exact: "1234.50").
// { n |> number }, { price |> number("style", "currency", "currency", "EUR") }.
func Number(n any, opts ...string) intl.Value { return intl.Number(decimal(n), first(opts)) }

// Duration is d with Intl.DurationFormat: { uptime |> duration }, { d |> duration("style", "long") }.
func Duration(d time.Duration, opts ...string) intl.Value { return intl.Duration(d, first(opts)) }

// List is items with Intl.ListFormat: { names |> list }, { options |> list("type", "disjunction") }.
func List(items []string, opts ...string) intl.Value { return intl.List(items, first(opts)) }

// Zone is a time zone's name in the language: { "Europe/Berlin" |> zone }.
func Zone(id string) intl.Value { return intl.Zone(id) }

// Intl is any Intl call with v as its argument, new Intl[api](lang, opts)[method](v), or with intl.Args as its
// arguments: { "JP" |> intl("DisplayNames", "of", "type", "region") }, { 2 |> intl("PluralRules", "select") },
// { intl.Args{3, 5} |> intl("NumberFormat", "formatRange") }, { intl.Args{} |> intl("Locale", "getWeekInfo") }.
func Intl(v any, api, method string, opts ...string) intl.Value {
	if a, ok := v.(intl.Args); ok {
		args := make([]string, len(a))
		for i, x := range a {
			args[i] = argJSON(x)
		}
		return intl.Of(api, method, first(opts), args...)
	}
	return intl.Of(api, method, first(opts), argJSON(v))
}

// Localize is an in-app path in the request's locale (intl.Site): href={ "/about" |> localize } is "/de/about" in
// German. Whole literals too: hx-post={ f`/cart/add?id=@{id}` |> localize }.
func Localize(ctx context.Context, path string) string { return intl.URL(ctx, path) }

// Url is a page's address in the request's locale: href={ pages.Board{Topic: t} |> url } is
// "/de/board?topic=games" in German. A page is any type with a Path, its address without the locale prefix: an app's
// page table, as structpages' URLFor reads one (gsxui's site), and with no reflection, so TinyGo builds it.
func Url(ctx context.Context, page interface{ Path() string }) string {
	return intl.URL(ctx, page.Path())
}

// Plain is a Value's text alone, for an attribute (which takes no markup): title={ v |> plain }.
func Plain(v intl.Value) intl.Plain { return v.Plain() }

// Text is a message's or a Value's text in the request's language, for where a renderer can't reach: a component
// input typed string (gsxui's Toast title={ locales.GreetToastTitle |> text }).
func Text(ctx context.Context, x intl.Textual) string { return intl.Text(ctx, x) }

func first(s []string) string { return intl.Options(s...) }

// decimal is n as a plain decimal string, exactly.
func decimal(n any) string {
	switch x := n.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	}
	b, _ := json.Marshal(n)
	return string(b)
}

// argJSON is v as an Intl argument: a string quoted, an instant in epoch milliseconds, a number as it is.
func argJSON(v any) string {
	switch x := v.(type) {
	case string:
		return intl.JSON(x)
	case time.Time:
		return intl.Millis(x)
	}
	return decimal(v)
}
