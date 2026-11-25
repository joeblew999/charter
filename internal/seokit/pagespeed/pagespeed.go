// Package pagespeed runs Lighthouse on a live page through Google's PageSpeed Insights API: on Google's servers, so
// no Node and no local Chrome. It reads the scores of its four categories (performance, accessibility, best
// practices, SEO), every audit that fails, and, when Chrome's users visit the page often enough, their Core Web Vitals.
package pagespeed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
)

// Scope is the OAuth scope PageSpeed Insights takes from a service account's token (searchconsole.Key.Token). The
// API has to be enabled in the account's Google Cloud project (mise-seo setup does).
const Scope = "openid"

// Auth is how a run identifies itself to PageSpeed Insights. An API key is all it needs: a free one, from the Google
// Cloud console (APIs & Services → Credentials → API key, restricted to the PageSpeed Insights API). A service
// account's token (Scope) works too. With neither, Google's small anonymous quota runs out quickly.
type Auth struct {
	Key   string // sent as ?key=
	Token string // sent as Authorization: Bearer
}

// Report is one Lighthouse run.
type Report struct {
	URL        string     `json:"url"`
	Strategy   string     `json:"strategy"` // mobile or desktop
	Categories []Category `json:"categories"`
	// Field is real Chrome users' experience of the page over 28 days (the Chrome UX Report): each metric's 75th
	// percentile and Google's verdict (FAST, AVERAGE, SLOW). Empty while the page has too few visits.
	Field []Metric `json:"field"`
}

// Category is a Lighthouse category: its score out of 100 and the audits that cost it.
type Category struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Score  int     `json:"score"`
	Failed []Audit `json:"failed"`
}

// Audit is one Lighthouse check that didn't pass.
type Audit struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Score   int      `json:"score"`
	Display string   `json:"display,omitempty"` // e.g. "2.8 s", "Potential savings of 120 KiB"
	Items   []string `json:"items,omitempty"`   // what fails: up to five elements (HTML) or resources (URLs)
}

// Metric is one Core Web Vital from real users.
type Metric struct {
	ID         string `json:"id"` // e.g. LARGEST_CONTENTFUL_PAINT_MS
	Percentile int    `json:"percentile"`
	Category   string `json:"category"`
}

// Endpoint is the PageSpeed Insights API.
var Endpoint = "https://www.googleapis.com/pagespeedonline/v5/runPagespeed"

// Run runs Lighthouse on page (an absolute URL) as strategy ("mobile", Google's index; or "desktop"), as auth. It
// takes Google 10 to 60 seconds.
func Run(ctx context.Context, httpc *http.Client, auth Auth, page, strategy string) (Report, error) {
	q := url.Values{"url": {page}, "strategy": {strategy}}
	if auth.Key != "" {
		q.Set("key", auth.Key)
	}
	for _, c := range []string{"PERFORMANCE", "ACCESSIBILITY", "BEST_PRACTICES", "SEO"} {
		q.Add("category", c)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", Endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return Report{}, err
	}
	if auth.Token != "" {
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	}
	res, err := httpc.Do(req)
	if err != nil {
		return Report{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return Report{}, err
	}
	var raw struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		LighthouseResult struct {
			FinalURL   string `json:"finalDisplayedUrl"`
			Categories map[string]struct {
				Title     string   `json:"title"`
				Score     *float64 `json:"score"`
				AuditRefs []struct {
					ID     string  `json:"id"`
					Weight float64 `json:"weight"`
				} `json:"auditRefs"`
			} `json:"categories"`
			Audits map[string]struct {
				Title        string          `json:"title"`
				Score        *float64        `json:"score"`
				Mode         string          `json:"scoreDisplayMode"`
				DisplayValue string          `json:"displayValue"`
				Details      json.RawMessage `json:"details"` // shaped by the audit: read leniently (items)
			} `json:"audits"`
		} `json:"lighthouseResult"`
		LoadingExperience struct {
			Metrics map[string]struct {
				Percentile int    `json:"percentile"`
				Category   string `json:"category"`
			} `json:"metrics"`
		} `json:"loadingExperience"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Report{}, fmt.Errorf("PageSpeed Insights: HTTP %d: %v: %.200s", res.StatusCode, err, body)
	}
	if raw.Error != nil {
		return Report{}, fmt.Errorf("PageSpeed Insights: %d %s", raw.Error.Code, raw.Error.Message)
	}
	r := Report{URL: page, Strategy: strategy, Categories: []Category{}, Field: []Metric{}}
	lr := raw.LighthouseResult
	for _, id := range []string{"performance", "accessibility", "best-practices", "seo"} {
		c, ok := lr.Categories[id]
		if !ok {
			continue
		}
		cat := Category{ID: id, Title: c.Title, Score: percent(c.Score), Failed: []Audit{}}
		for _, ref := range c.AuditRefs {
			a := lr.Audits[ref.ID]
			// A failing check that counts (weight), or an opportunity that would save load time (metricSavings).
			if a.Score == nil || *a.Score >= 0.9 || !(ref.Weight > 0 && (a.Mode == "binary" || a.Mode == "numeric") || a.Mode == "metricSavings") {
				continue
			}
			failed := Audit{ID: ref.ID, Title: a.Title, Score: percent(a.Score), Display: a.DisplayValue}
			for _, raw := range items(a.Details) {
				if len(failed.Items) == 5 {
					break
				}
				if it := item(raw); it != "" {
					failed.Items = append(failed.Items, it)
				}
			}
			cat.Failed = append(cat.Failed, failed)
		}
		r.Categories = append(r.Categories, cat)
	}
	for id, m := range raw.LoadingExperience.Metrics {
		r.Field = append(r.Field, Metric{ID: id, Percentile: m.Percentile, Category: m.Category})
	}
	sort.Slice(r.Field, func(i, j int) bool { return r.Field[i].ID < r.Field[j].ID })
	return r, nil
}

func percent(score *float64) int {
	if score == nil {
		return 0
	}
	return int(*score*100 + 0.5)
}

// items are an audit's detail items, when they are a list.
func items(details json.RawMessage) []json.RawMessage {
	var d struct {
		Items json.RawMessage `json:"items"`
	}
	var list []json.RawMessage
	if json.Unmarshal(details, &d) != nil || json.Unmarshal(d.Items, &list) != nil {
		return nil
	}
	return list
}

// item is what one audit detail names: an element's HTML, else a message (a console error) with where it came
// from, else a resource's URL, else "".
func item(raw json.RawMessage) string {
	var it struct {
		URL         any    `json:"url"`
		Description string `json:"description"`
		Node        struct {
			Snippet string `json:"snippet"`
		} `json:"node"`
	}
	if json.Unmarshal(raw, &it) != nil {
		return ""
	}
	u, _ := it.URL.(string)
	switch {
	case it.Node.Snippet != "":
		return it.Node.Snippet
	case it.Description != "" && u != "":
		return it.Description + " (" + u + ")"
	case it.Description != "":
		return it.Description
	}
	return u
}
