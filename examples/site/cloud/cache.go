//go:build js && wasm

package cloud

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/syumai/workers-go/cloudflare/cache"

	"github.com/joeblew999/charter/transport"
)

// The Cache API: a value stored under an address of this site, for a minute. The first visit stores it.

func cached(r *http.Request) string {
	at, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, transport.Origin(r)+"/cloud/cached", nil)
	c := cache.New()
	if hit, err := c.Match(at, nil); err == nil {
		b, _ := io.ReadAll(hit.Body)
		return "hit: stored " + string(b)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	// The length is given: workers-go then writes the body to a fixed-length stream.
	err := c.Put(at, &http.Response{
		StatusCode: 200, ContentLength: int64(len(now)),
		Header: http.Header{"Cache-Control": {"max-age=60"}, "Content-Type": {"text/plain"}},
		Body:   io.NopCloser(strings.NewReader(now)),
	})
	if err != nil {
		return err.Error()
	}
	return "miss: stored " + now
}
