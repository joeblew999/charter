package command

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// A page that is the same for every reader is answered by Cloudflare's cache for the Worker, before the Worker runs
// (app's shared.go). A deploy starts an empty cache, so the first reader of each page would wait for the Worker to
// start and draw it. keep asks for every page of the sitemap once, as a browser does that has no cookie, so the
// cache has them when a reader comes. It fails nothing: a page not kept is drawn for its reader, as before.
func keep(address string) {
	if address == "" {
		return
	}
	client := &http.Client{Timeout: 20 * time.Second}
	ask := func(url string, navigation bool) (*http.Response, string) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, ""
		}
		if navigation {
			req.Header.Set("Accept", "text/html")
			req.Header.Set("Sec-Fetch-Mode", "navigate")
			req.Header.Set("Sec-Fetch-Dest", "document")
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, ""
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res, string(body)
	}
	res, sitemap := ask(strings.TrimSuffix(address, "/")+"/sitemap.xml", false)
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
	var (
		wait  sync.WaitGroup
		lock  sync.Mutex
		kept  int
		limit = make(chan struct{}, 6)
	)
	for _, page := range pages {
		wait.Add(1)
		go func() {
			defer wait.Done()
			limit <- struct{}{}
			defer func() { <-limit }()
			if res, _ := ask(page, true); res != nil && strings.HasPrefix(res.Header.Get("Cache-Control"), "public") && res.Header.Get("Cf-Cache-Status") != "" {
				lock.Lock()
				kept++
				lock.Unlock()
			}
		}()
	}
	wait.Wait()
	if kept > 0 {
		say("Cloudflare's cache has %d of the sitemap's %d pages: the ones that are the same for every reader, answered from here on before the Worker runs", kept, len(pages))
	}
}

var sitemapPage = regexp.MustCompile(`<loc>([^<]+)</loc>`)
