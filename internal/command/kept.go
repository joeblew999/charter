package command

import (
	"cmp"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// A page that is the same for every reader is answered by Cloudflare's cache for the Worker, before the Worker runs
// (app's shared.go). A deploy starts an empty cache, so the first reader of each page would wait for the Worker to
// start and draw it. keep asks for every page of the sitemap as a reader with no cookie does, so the cache has them
// when one comes: once as a page and once as a link followed in the app (htmx's boosted link names its target), the
// two copies the cache keeps of a page (shared.go's Vary).
//
// It is also where a deploy is seen to have emptied the cache. Every answer says the build that drew it
// (X-Charter-Build, build.go), a kept one too: a page answered with another build than the one just deployed is a
// copy of before it, and the deploy fails with its address. A page not kept fails nothing: it is drawn for its
// reader, as before.
func keep(address, build string) {
	if address == "" {
		return
	}
	client := &http.Client{Timeout: 20 * time.Second}
	ask := func(url string, as map[string]string) (*http.Response, string) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, ""
		}
		for name, value := range as {
			req.Header.Set(name, value)
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, ""
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res, string(body)
	}
	res, sitemap := ask(strings.TrimSuffix(address, "/")+"/sitemap.xml", nil)
	if res == nil || res.StatusCode != http.StatusOK {
		return
	}
	var pages []string
	seen := map[string]bool{}
	for _, loc := range sitemapPage.FindAllStringSubmatch(sitemap, -1) {
		if !seen[loc[1]] {
			seen[loc[1]] = true
			pages = append(pages, loc[1])
		}
	}
	// pass asks for every page both ways: how many Cloudflare keeps, and which were answered by another build.
	pass := func() (kept int, older []string) {
		var (
			wait  sync.WaitGroup
			lock  sync.Mutex
			limit = make(chan struct{}, 6)
		)
		for _, page := range pages {
			wait.Add(1)
			go func() {
				defer wait.Done()
				limit <- struct{}{}
				defer func() { <-limit }()
				link, _ := ask(page, asLink)
				res, _ := ask(page, asLoad)
				lock.Lock()
				defer lock.Unlock()
				for _, answer := range []*http.Response{link, res} {
					if answer != nil && build != "" && answer.Header.Get("X-Charter-Build") != build {
						_, colo, _ := strings.Cut(answer.Header.Get("Cf-Ray"), "-")
						older = append(older, fmt.Sprintf("%s: HTTP %d, build %s, Cf-Cache-Status %s, age %s, at %s", page, answer.StatusCode,
							cmp.Or(answer.Header.Get("X-Charter-Build"), "unsaid"), cmp.Or(answer.Header.Get("Cf-Cache-Status"), "unsaid"), cmp.Or(answer.Header.Get("Age"), "0"), colo))
						return
					}
				}
				if res != nil && strings.HasPrefix(res.Header.Get("Cache-Control"), "public") && res.Header.Get("Cf-Cache-Status") != "" {
					kept++
				}
			}()
		}
		wait.Wait()
		return kept, older
	}
	// A new version reaches Cloudflare's data centres over some seconds, and until it has reached one the version
	// before it answers there, with its own cache (seen on 2026-11-28: a kept page of the build before, 24 s old,
	// from one data centre of three). So the pages are asked until every one is of this build, for a minute.
	started := time.Now()
	kept, older := pass()
	for len(older) > 0 && time.Since(started) < time.Minute {
		time.Sleep(3 * time.Second)
		kept, older = pass()
	}
	if len(older) > 0 {
		slices.Sort(older)
		fmt.Fprintf(os.Stderr, "✗ %d of the sitemap's %d pages are still not answered by this build (%s), a minute after it was deployed: a copy of before this deploy is given.\n  %s\n", len(older), len(pages), build, strings.Join(older, "\n  "))
		os.Exit(1)
	}
	if build != "" {
		say("every page of the sitemap is answered by this build (%s), %.0f s after it was deployed: nothing kept is of before it", build, time.Since(started).Seconds())
	}
	if kept > 0 {
		say("Cloudflare's cache has %d of the sitemap's %d pages: the ones that are the same for every reader, answered from here on before the Worker runs", kept, len(pages))
	}
}

var sitemapPage = regexp.MustCompile(`<loc>([^<]+)</loc>`)

// The two ways a kept page is asked for: with no target, and by htmx 4's boosted link, whose target is the body.
var (
	asLoad = map[string]string{"Accept": "text/html"}
	asLink = map[string]string{"Accept": "text/html", "HX-Request": "true", "HX-Target": "body"}
)
