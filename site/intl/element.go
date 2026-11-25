package intl

import "context"

// Attr is one HTML attribute: its name and value.
type Attr struct{ Key, Value string }

// Element is how v is written in HTML, for whatever renders it (gsxintl.Render in a gsx app, or an app's own
// templates): its tag, its attributes, and its text in the request's language, time zone and clock. The attributes are
// the call js/intl-browser.mjs redoes in the viewer's own zone: <time datetime> for instants, <data value> for numbers
// and durations, else <span>.
func (v Value) Element(ctx context.Context) (tag string, attrs []Attr, text string) {
	text, call := v.text(ctx), v.Attrs()
	switch {
	case v.kind == "since": // its call carries its datetime
		return "time", call, text
	case v.tag == "time":
		return "time", append([]Attr{{"datetime", v.machine}}, call...), text
	case v.tag == "data":
		return "data", append([]Attr{{"value", v.machine}}, call...), text
	}
	return "span", call, text
}

// Attrs is v's call as js/intl-browser.mjs reads it, for an element that can't hold Element's (an <option>): its
// attributes, with v.Plain() as the element's text.
func (v Value) Attrs() []Attr {
	switch v.kind {
	case "since":
		return []Attr{{"datetime", v.machine}, {"data-intl", "since"}}
	case "zone":
		return []Attr{{"data-intl", "zone"}, {"data-args", "[" + JSON(v.zone) + "]"}}
	}
	a := []Attr{{"data-intl", v.API}}
	if v.Method != "" {
		a = append(a, Attr{"data-method", v.Method})
	}
	if v.Opts != "" {
		a = append(a, Attr{"data-opts", v.Opts})
	}
	return append(a, Attr{"data-args", v.ArgsJSON()})
}

// HTML is the <html> element's language attributes for the request: lang and dir from its locale, and the viewer's
// chosen time zone and clock (data-tz, data-hc) for js/intl-browser.mjs.
func HTML(ctx context.Context) []Attr {
	l := Current(ctx)
	return []Attr{{"lang", l.ID()}, {"dir", l.Dir()}, {"data-tz", ChosenTimeZone(ctx)}, {"data-hc", HourCycle(ctx)}}
}

// Shipped is every locale the app ships, the source first (Use set them).
func Shipped() Locales { return shipped }
