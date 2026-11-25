package intl

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// The reader's language: the one they chose, else the one their browser asks for (Accept-Language). On a page in
// another, Site puts it in the request, and the app offers that version (Suggested) until they choose. Never a
// redirect: Googlebot sends no Accept-Language, and a redirect would hide the other language versions from it.

// Suggested is the request's page in the language the reader wants, when the site ships it and the page is in
// another: the one they chose, else the one their browser asks for first. The app links to it, written in that
// language (Msg.In), with Decline beside it. ok is false otherwise.
func Suggested(ctx context.Context) (Link, bool) {
	r, ok := FromContext(ctx)
	if !ok || r.Preferred == nil || r.Preferred == r.Locale {
		return Link{}, false
	}
	for _, l := range Links(ctx) {
		if l.Lang == r.Preferred.ID() {
			return l, true
		}
	}
	return Link{}, false
}

// Decline is the link that turns Suggested's offer down: this page, its language kept as the reader's choice (Site
// redirects it to the page's clean URL), so the offer stops.
func Decline(ctx context.Context) string {
	path := "/"
	if r, ok := FromContext(ctx); ok && r.Path != "" {
		path = r.Path
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return Prefix(Current(ctx)) + path + sep + LocaleCookie + "=" + Current(ctx).ID()
}

// preferred is the shipped locale Accept-Language asks for first, matched the way Intl matches: by language and
// script after Intl.Locale's maximize ("de-AT" is de, "zh-TW" is zh-Hant, "pt" is pt-BR), the region breaking a tie
// between two shipped versions of one language. nil when none matches.
func preferred(header string) *Locale {
	for _, tag := range acceptLanguage(header) {
		if l := shipped.Find(tag); l != nil {
			return l
		}
		lang, script, region := subtags(maximal(tag))
		if lang == "" {
			continue
		}
		var match *Locale
		for _, l := range shipped {
			sl, ss, sr := subtags(l.Maximal())
			if sl != lang || ss != script {
				continue
			}
			if sr == region {
				return l
			}
			if match == nil {
				match = l
			}
		}
		if match != nil {
			return match
		}
	}
	return nil
}

// acceptLanguage is an Accept-Language header's language tags, most wanted first: q=0 and "*" dropped, at most eight,
// each a plausible BCP 47 tag (it comes from anyone).
func acceptLanguage(header string) []string {
	type want struct {
		tag string
		q   float64
	}
	var wants []want
	for _, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				q = f
			}
		}
		if q <= 0 || tag == "*" || tag == "" || len(tag) > 35 || strings.Trim(tag, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-") != "" {
			continue
		}
		wants = append(wants, want{tag, q})
		if len(wants) == 8 {
			break
		}
	}
	slices.SortStableFunc(wants, func(a, b want) int { return cmp.Compare(b.q, a.q) })
	out := make([]string, len(wants))
	for i, w := range wants {
		out[i] = w.tag
	}
	return out
}

// maximals is maximal's cache: the same few tags arrive on every request. At most 512 are kept.
var maximals sync.Map

// maximal is tag with Intl.Locale's maximize ("zh-TW" → "zh-Hant-TW"), "" when Intl can't read it.
func maximal(tag string) string {
	if m, ok := maximals.Load(tag); ok {
		return m.(string)
	}
	m, _ := NewLocale(tag).Intl(Call{API: "Locale", Method: "maximize"}, "", "")
	n := 0
	maximals.Range(func(any, any) bool { n++; return n < 512 })
	if n < 512 {
		maximals.Store(tag, m)
	}
	return m
}

// subtags are a maximal tag's language, script and region ("zh-Hant-TW"), its extensions dropped.
func subtags(tag string) (lang, script, region string) {
	tag, _, _ = strings.Cut(tag, "-u-")
	parts := strings.Split(tag, "-")
	if len(parts) < 3 {
		return "", "", ""
	}
	return strings.ToLower(parts[0]), parts[1], strings.ToUpper(parts[2])
}
