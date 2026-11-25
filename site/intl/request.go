package intl

import "context"

type ctxKey struct{}

// Request is the request-scoped state an app puts in the context: the resolved locale, the request
// path without its locale prefix (so views can link the same page in another locale), and the viewer's
// date and time preferences.
type Request struct {
	Locale *Locale
	Path   string // e.g. "/about?x=1", never locale-prefixed
	// Origin is the request's scheme and host, e.g. "https://example.com", for absolute URLs: canonical links,
	// hreflang alternates and sitemaps must not be relative.
	Origin string

	// TimeZone is the viewer's IANA time zone ("Europe/Berlin"), "" for unknown: the browser formats dates in it.
	TimeZone string
	// TimeZoneChosen is true when TimeZone is the viewer's explicit choice rather than a guess (e.g. from
	// the connection), so browser code must not replace it with the browser's zone.
	TimeZoneChosen bool
	// HourCycle is the viewer's explicit clock, "h12" or "h23"; "" keeps the locale's.
	HourCycle string
	// Preferred is the shipped locale the reader wants, on a page view: the one they chose (ChosenLocaleCookie), else
	// the one their browser asks for first (Accept-Language); nil when neither. Suggested offers the page in it.
	Preferred *Locale
	// Decision is how Site decided the locale and zone, for showing it (DecisionOf).
	Decision Decision
}

// Decision is everything Site decided a request's language and time zone from, and what won: for a page that shows
// a reader why it is in their language and time.
type Decision struct {
	LanguageFrom   string // "url" (the address's prefix, /de/…) or "default" (no prefix: the source locale)
	AcceptLanguage string // what the reader's browser asks for
	LastViewed     string // the locale of the last page they viewed (the locale cookie); "" on a first visit
	ChosenLanguage string // the locale they chose (the chosen_locale cookie, or this page view), "" for none
	ZoneFrom       string // where TimeZone came from: "chosen", "browser", "connection" or "default" (UTC)
	ChosenZone     string // the zone the reader chose (the tz cookie), "" for none
	BrowserZone    string // the zone their browser reported (the browser_tz cookie), "" until it has
	ConnectionZone string // the connection's guess (Cloudflare's, from the IP), "" for none
}

// DecisionOf is how the request's language and zone were decided; the zero Decision without a request.
func DecisionOf(ctx context.Context) Decision {
	r, _ := FromContext(ctx)
	return r.Decision
}

// WithRequest returns ctx carrying r. The app's locale middleware calls it once per request; the renderers (Value,
// Msg) and components read it through the render context with [FromContext].
func WithRequest(ctx context.Context, r Request) context.Context {
	return context.WithValue(ctx, ctxKey{}, r)
}

// Current is the request's locale: <html lang={ intl.Current(ctx).ID() } dir={ intl.Current(ctx).Dir() }>. A render
// without a request is in the source locale.
func Current(ctx context.Context) *Locale {
	if r, ok := FromContext(ctx); ok {
		return r.Locale
	}
	return Default()
}

// FromContext returns the request's intl state; ok is false when none was set.
func FromContext(ctx context.Context) (r Request, ok bool) {
	r, ok = ctx.Value(ctxKey{}).(Request)
	return r, ok && r.Locale != nil
}
