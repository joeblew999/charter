package intl

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// Value is an Intl call written in markup where its result goes: gsx renders it (Render, registered in gsx.toml
// [renderers]) with the request's language, time zone and clock, inside an element js/intl-browser.mjs redoes in
// the viewer's own zone:
//
//	<p>{ intl.Date(note.CreatedAt, intl.Options("dateStyle", "full")) }</p>
//	<p>{ intl.Of("NumberFormat", "", intl.Options("style", "percent"), intl.JSON("0.256")) }</p>
type Value struct {
	Call
	kind     string // "" for the Call, "since" or "zone"
	tag      string // "time", "data" or "span"
	machine  string // the time element's datetime, the data element's value
	fallback string // where Intl can't: the value's machine form (an ISO instant, a decimal, JSON), never Go's formatting
	since    time.Time
	zone     string
}

// Args are an Intl call's arguments when it takes several (or none), for the intl filter:
// { intl.Args{"ä", "z"} |> intl("Collator", "compare") }. Each is a string, a number or a time.Time.
type Args []any

// Of is any Intl call: new Intl[api](lang, opts)[method](...args), method "" for "format", opts a JSON object, args
// JSON values (JSON for strings, Millis for instants).
func Of(api, method, opts string, args ...string) Value {
	return Value{Call: Call{API: api, Method: method, Opts: opts, Args: args}, tag: "span", fallback: strings.Join(args, " ")}
}

// Date is t with Intl.DateTimeFormat and opts ("" for the language's default date).
func Date(t time.Time, opts string) Value {
	return Value{Call: Call{API: "DateTimeFormat", Opts: opts, Args: []string{Millis(t)}}, tag: "time", machine: ISO(t), fallback: ISO(t)}
}

// DateRange is a to b with Intl.DateTimeFormat's formatRange.
func DateRange(a, b time.Time, opts string) Value {
	return Value{Call: Call{API: "DateTimeFormat", Method: "formatRange", Opts: opts, Args: []string{Millis(a), Millis(b)}},
		tag: "time", machine: ISO(a), fallback: ISO(a) + "/" + ISO(b)}
}

// Since is t relative to now ("3 minutes ago", Intl.RelativeTimeFormat), kept fresh in the browser.
func Since(t time.Time) Value {
	return Value{kind: "since", tag: "time", machine: ISO(t), fallback: ISO(t), since: t}
}

// Number is a plain decimal ("1234.5", exact: never a float) with Intl.NumberFormat and opts.
func Number(n, opts string) Value {
	return Value{Call: Call{API: "NumberFormat", Opts: opts, Args: []string{JSON(n)}}, tag: "data", machine: n, fallback: n}
}

// Int is n with Intl.NumberFormat: the language's digits and grouping.
func Int(n int64) Value { return Number(strconv.FormatInt(n, 10), "") }

// Duration is d with Intl.DurationFormat and opts, in days, hours, minutes and seconds. Under a second it asks for
// secondsDisplay "always": Intl hides zero units, and would say nothing at all.
func Duration(d time.Duration, opts string) Value {
	s := int64(d / time.Second)
	j := `{"days":` + strconv.FormatInt(s/86400, 10) + `,"hours":` + strconv.FormatInt(s/3600%24, 10) +
		`,"minutes":` + strconv.FormatInt(s/60%60, 10) + `,"seconds":` + strconv.FormatInt(s%60, 10) + `}`
	if s == 0 {
		if o := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(opts), "{"), "}")); o != "" {
			opts = `{"secondsDisplay":"always",` + o + `}`
		} else {
			opts = `{"secondsDisplay":"always"}`
		}
	}
	return Value{Call: Call{API: "DurationFormat", Opts: opts, Args: []string{j}}, tag: "data", machine: j, fallback: j}
}

// List is items with Intl.ListFormat and opts.
func List(items []string, opts string) Value {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = JSON(s)
	}
	list := "[" + strings.Join(q, ",") + "]"
	return Value{Call: Call{API: "ListFormat", Opts: opts, Args: []string{list}}, tag: "span", fallback: list}
}

// Zone is a time zone's name in the language: "Europe/Berlin · Mitteleuropäische Zeit (GMT+2)".
func Zone(id string) Value { return Value{kind: "zone", tag: "span", fallback: id, zone: id} }

// Clock is 13:05 written in a clock, to label the choices of HourCycle: "h12", "h23", or "" for the language's own,
// whichever clock the viewer chose. In an <option>: { gsxintl.Attrs(intl.Clock("h12"))... } and { intl.Clock("h12") |> plain }.
func Clock(hourCycle string) Value {
	clock := []string{"viewerClock", "false"} // js/intl.mjs: the language's own clock, not the viewer's
	if hourCycle != "" {
		clock = []string{"hourCycle", hourCycle}
	}
	opts := Options(append([]string{"hour", "numeric", "minute", "2-digit", "timeZone", "UTC"}, clock...)...)
	return Date(time.Date(2026, 1, 1, 13, 5, 0, 0, time.UTC), opts)
}

// Textual is a Msg or a Value: what has a text in the request's language.
type Textual interface{ text(context.Context) string }

// Text is a Value's or a Msg's text in the request's language, time zone and clock, for Go code (in markup, the
// text filter). A Value where Intl can't is its fallback.
func Text(ctx context.Context, x Textual) string { return x.text(ctx) }

func (m Msg) text(ctx context.Context) string { return Translate(ctx, m) }

func (v Value) text(ctx context.Context) string {
	r, ok := FromContext(ctx)
	if !ok {
		r = Request{Locale: Default()}
	}
	var s string
	switch v.kind {
	case "since":
		s, ok = r.Locale.Since(v.since)
	case "zone":
		s, ok = r.Locale.ZoneName(v.zone)
	default:
		s, ok = r.Locale.Intl(v.Call, r.TimeZone, r.HourCycle)
	}
	if !ok {
		return v.fallback
	}
	return s
}

// Slot marks where js/intl-browser.mjs puts a value in an element's data-template: a translated message rendered with
// Slot as its argument, so the browser's value lands inside the translator's wording:
//
//	<option data-intl="browser-zone" data-template={ locales.Automatic(intl.Slot) }>…</option>
const Slot = "\ue000"

// Plain is a Value as plain text (PlainText, registered in gsx.toml [renderers]): for attributes, which take no
// markup, and for text the browser leaves alone. A type has one renderer, so this is a second type (the gsx
// Translation keys pattern).
type Plain Value

// Plain is v as plain text.
func (v Value) Plain() Plain { return Plain(v) }

// PlainText is Plain's gsx renderer: the Intl result in the request's language, time zone and clock.
func PlainText(ctx context.Context, p Plain) string { return Value(p).text(ctx) }

// ISO is t for a <time datetime>: UTC RFC 3339, with milliseconds when it has them.
func ISO(t time.Time) string {
	if t.Nanosecond() != 0 {
		return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
	}
	return t.UTC().Format(time.RFC3339)
}
