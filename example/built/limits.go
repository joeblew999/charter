//go:build js && wasm

package built

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/syumai/workers-go/cloudflare/cache"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/db"
	"github.com/joeblew999/charter/example/env"
)

// A rate limit, and the Cache API: the two ways a Worker does less.

// UseLimit makes one call against the write limit as key (a caller, an address), and says whether it was within it.
// The limit is the one the API's createNote is under (WRITE_LIMIT); Cloudflare says yes or no, never how many are
// left.
func UseLimit(key string) (Limit, error) {
	ok, err := env.WRITE_LIMIT.Allow(key)
	return Limit{Calls: env.WRITE_LIMIT.Calls(), Seconds: env.WRITE_LIMIT.Seconds(), Asked: true, Allowed: ok}, err
}

// ReportNow is the notes in numbers. Working it out takes a query; the answer is kept in the cache for a minute,
// under an address of this site, and given from there until then.
func ReportNow(r *http.Request) (Report, error) {
	start := time.Now()
	at, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, app.Origin(r)+"/built/limits/report.json", nil)
	c := cache.New()
	if hit, err := c.Match(at, nil); err == nil {
		var report Report
		b, _ := io.ReadAll(hit.Body)
		if json.Unmarshal(b, &report) == nil {
			report.FromCache, report.Took = true, time.Since(start)
			return report, nil
		}
	}
	conn, err := env.DB.Open()
	if err != nil {
		return Report{}, err
	}
	row, err := db.NotesReport(conn)
	if err != nil {
		return Report{}, err
	}
	report := Report{Notes: row.Notes, Authors: row.Authors, At: time.Now().UTC().Truncate(time.Second)}
	body, _ := json.Marshal(report)
	// The length is given: workers-go then writes the body to a fixed-length stream.
	if err := c.Put(at, &http.Response{
		StatusCode: 200, ContentLength: int64(len(body)),
		Header: http.Header{"Cache-Control": {"max-age=60"}, "Content-Type": {"application/json"}},
		Body:   io.NopCloser(strings.NewReader(string(body))),
	}); err != nil {
		return Report{}, err
	}
	report.Took = time.Since(start)
	return report, nil
}
