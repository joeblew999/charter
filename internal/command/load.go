package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joeblew999/charter/internal/project"
)

// charter run load asks the Worker's pages many at once for a few seconds, with readers' streams held open beside
// them, and says how it did: how many it answered a second, how fast, and whether Cloudflare ended any of them. It
// is for before a release, or when something about what the Worker holds has changed; it is no part of a deploy.
//
// What a Go Worker does wrong under load it does only on Cloudflare, where its memory and its CPU time are limited,
// and only when requests come together: so this is asked of the deployed Worker, and tests of one request at a time
// do not find it.
//
//	charter run load --deployed             the deployed Worker: 20 at once for 10 seconds, 10 streams open
//	charter run load staging                an environment
//	charter run load --deployed -at 50 -for 30s -streams 40
//	charter run load                        the Worker on wrangler dev: how fast, with none of Cloudflare's limits
//
// It only reads: the pages of the sitemap, and the streams the first of them opens.
func loadTask(p *project.Project, args []string) {
	env := takeEnv(p, &args)
	args = slices.Clone(args)
	deployed := env != ""
	if i := slices.Index(args, "--deployed"); i >= 0 {
		args, deployed = slices.Delete(args, i, i+1), true
	}
	fs := flag.NewFlagSet("charter run load", flag.ExitOnError)
	at := fs.Int("at", 20, "how many requests at once")
	lasts := fs.Duration("for", 10*time.Second, "for how long")
	streams := fs.Int("streams", 10, "how many readers' streams to hold open meanwhile")
	fs.Parse(args)
	if fs.NArg() > 0 || *at < 1 || *lasts < time.Second {
		check(errors.New("usage: charter run load [env | --deployed] [-at 20] [-for 10s] [-streams 10]"))
	}
	if !deployed {
		check(serving(p, func(base string) error { return loaded(p, base, "", false, *at, *streams, *lasts) }))
		return
	}
	base := p.URL
	if env != "" {
		base = p.URLs[env]
	}
	if base == "" {
		check(errors.New(`charter.toml has no address for it: url = "https://…" (go tool charter deploy writes it)`))
	}
	check(loaded(p, strings.TrimRight(base, "/"), env, true, *at, *streams, *lasts))
}

var (
	sitemapLoc = regexp.MustCompile(`<loc>([^<]+)</loc>`)
	streamOf   = regexp.MustCompile(`hx-sse:connect="([^"]+)"`)
)

// loaded loads the Worker at base and says how it did.
func loaded(p *project.Project, base, env string, deployed bool, at, streams int, lasts time.Duration) error {
	client := &http.Client{Timeout: 30 * time.Second}
	get := func(ctx context.Context, address string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "charter-load")
		return client.Do(req)
	}
	// What to ask: the pages the sitemap names, at this address.
	pages := []string{base + "/"}
	if res, err := get(context.Background(), base+"/sitemap.xml"); err == nil {
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		var found []string
		for _, m := range sitemapLoc.FindAllStringSubmatch(string(b), -1) {
			if i := strings.Index(m[1], "://"); i >= 0 {
				if j := strings.Index(m[1][i+3:], "/"); j >= 0 {
					found = append(found, base+html.UnescapeString(m[1][i+3+j:]))
				}
			}
		}
		if len(found) > 0 {
			pages = found
		}
	}
	// And the streams its pages open, held as a reader's browser holds them.
	var feeds []string
	for _, page := range pages[:min(len(pages), 8)] {
		res, err := get(context.Background(), page)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		for _, m := range streamOf.FindAllStringSubmatch(string(b), -1) {
			if feed := base + html.UnescapeString(m[1]); !slices.Contains(feeds, feed) {
				feeds = append(feeds, feed)
			}
		}
	}
	if len(feeds) == 0 {
		streams = 0
	}
	say("%s: %d pages asked %d at once for %s, with %d streams held open", base, len(pages), at, lasts, streams)

	began := time.Now()
	ctx, cancel := context.WithDeadline(context.Background(), began.Add(lasts))
	defer cancel()
	var held sync.WaitGroup
	for i := range streams {
		held.Go(func() {
			stream := &http.Client{} // no timeout: it is held until the load is over
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, feeds[i%len(feeds)], nil)
			req.Header.Set("Accept", "text/event-stream")
			if res, err := stream.Do(req); err == nil {
				io.Copy(io.Discard, res.Body)
				res.Body.Close()
			}
		})
	}
	var (
		mu       sync.Mutex
		took     []time.Duration
		statuses = map[int]int{}
		failed   int
		next     atomic.Int64
		asking   sync.WaitGroup
	)
	for range at {
		asking.Go(func() {
			for ctx.Err() == nil {
				page := pages[int(next.Add(1))%len(pages)]
				start := time.Now()
				// Its own time to finish: a request that began before the end is counted.
				res, err := get(context.Background(), page)
				d := time.Since(start)
				mu.Lock()
				if err != nil {
					failed++
				} else {
					io.Copy(io.Discard, res.Body)
					res.Body.Close()
					statuses[res.StatusCode]++
					took = append(took, d)
				}
				mu.Unlock()
			}
		})
	}
	asking.Wait()
	ended := time.Now()
	cancel()
	held.Wait()

	slices.Sort(took)
	total := len(took) + failed
	if len(took) == 0 {
		return errors.New("the Worker answered nothing")
	}
	say("%s requests, %.0f a second: half answered within %s, 99%% within %s",
		grouped(int64(total)), float64(total)/ended.Sub(began).Seconds(), took[len(took)/2].Round(time.Millisecond), took[len(took)*99/100].Round(time.Millisecond))
	wrong := failed
	codes := make([]int, 0, len(statuses))
	for code := range statuses {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	for _, code := range codes {
		if code >= 500 {
			say("✗ %d answered %d", statuses[code], code)
			wrong += statuses[code]
		} else if code >= 400 {
			say("· %d answered %d", statuses[code], code)
		}
	}
	if failed > 0 {
		say("✗ %d were not answered at all", failed)
	}
	if wrong == 0 {
		say("✓ every request was answered")
	}
	runtimesHold(base, p.HealthPath(), map[bool]string{true: "deployed", false: "local"}[deployed])
	if !deployed {
		say("· this was wrangler dev, which has none of Cloudflare's limits: go tool charter run load --deployed finds what they do")
	} else if deployedHere(p, base) {
		// What no answer shows: the requests Cloudflare ended.
		if err := healthOf(p, env, began, int64(total)); err != nil {
			return err
		}
	}
	if wrong > 0 {
		return fmt.Errorf("%d of %d requests went wrong under load", wrong, total)
	}
	return nil
}
