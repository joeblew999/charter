// Package test asks the Worker, on wrangler dev, for the Cloudflare page: BASE is its URL (go tool charter test sets
// it). Each card's binding is used through the page, as a reader would.
package test

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

var base = strings.TrimRight(os.Getenv("BASE"), "/")

// local says the Worker is wrangler dev on this machine, which has two things a deployed one has not: a Cron Trigger
// can be fired by hand, and an email can be handed to the Worker.
var local = strings.Contains(base, "://127.0.0.1") || strings.Contains(base, "://localhost")

func TestMain(m *testing.M) {
	if base == "" {
		fmt.Println("no BASE, the Worker's URL: go tool charter test")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func page(t *testing.T, path string) string {
	t.Helper()
	res, err := http.Get(base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("GET %s: HTTP %d\n%s", path, res.StatusCode, b)
	}
	return string(b)
}

// post sends a form as the page's own does, and follows the redirect back to it.
func post(t *testing.T, path string, form url.Values) {
	t.Helper()
	req, _ := http.NewRequest("POST", base+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", base+"/cloud")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || !strings.HasSuffix(res.Request.URL.Path, "/cloud") {
		t.Fatalf("POST %s: HTTP %d at %s, want the page again", path, res.StatusCode, res.Request.URL)
	}
}

// soon waits for the page to show what happens after an answer: a queue's consumer, a Cron Trigger.
func soon(t *testing.T, what, want string) {
	t.Helper()
	// On Cloudflare, KV answers a read from its cache for up to a minute, so what a consumer wrote shows that much later.
	patience := 30 * time.Second
	if !local {
		patience = 3 * time.Minute
	}
	for start := time.Now(); time.Since(start) < patience; time.Sleep(time.Second) {
		if strings.Contains(page(t, "/cloud"), want) {
			return
		}
	}
	t.Errorf("%s: the page never showed %q", what, want)
}

func TestKVCountsVisits(t *testing.T) {
	visits := regexp.MustCompile(`<p id="visits"[^>]*>(\d+)</p>`)
	a, b := visits.FindStringSubmatch(page(t, "/cloud")), visits.FindStringSubmatch(page(t, "/cloud"))
	if a == nil || b == nil || a[1] == b[1] {
		t.Errorf("visits: %v then %v, want a count that goes up", a, b)
	}
}

func TestR2KeepsTheNote(t *testing.T) {
	post(t, "/cloud/note", url.Values{"note": {"kept in a bucket"}})
	if !strings.Contains(page(t, "/cloud"), `value="kept in a bucket"`) {
		t.Error("the note is not on the page")
	}
}

func TestQueueReachesItsConsumer(t *testing.T) {
	post(t, "/cloud/queue", url.Values{"message": {"through the queue"}})
	soon(t, "the queue's consumer", ">through the queue</li>")
}

func TestCronTrigger(t *testing.T) {
	want := `<p id="cron" class="text-sm" translate="no">*/5 * * * *`
	if !local {
		// Deployed, Cloudflare fires it every five minutes; a schedule just deployed can take a quarter of an hour to
		// start. Once it has run, the page keeps showing it.
		for start := time.Now(); time.Since(start) < 16*time.Minute; time.Sleep(10 * time.Second) {
			if strings.Contains(page(t, "/cloud"), want) {
				return
			}
		}
		t.Fatal("the Cron Trigger did not run in sixteen minutes")
	}
	// wrangler dev --test-scheduled fires one by hand.
	res, err := http.Get(base + "/__scheduled?cron=*/5+*+*+*+*")
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("GET /__scheduled: %v %v", res, err)
	}
	res.Body.Close()
	soon(t, "the Cron Trigger", want)
}

func TestVariableAndFetch(t *testing.T) {
	p := page(t, "/cloud")
	for _, want := range []string{`translate="no">charter-site`, "GET https://example.com/ → 200"} {
		if !strings.Contains(p, want) {
			t.Errorf("the page has no %q", want)
		}
	}
}

func TestTheGermanPage(t *testing.T) {
	if p := page(t, "/de/cloud"); !strings.Contains(p, "<h1") || !strings.Contains(p, "Was Cloudflare diesem Worker bereitstellt") {
		t.Error("/de/cloud is not in German")
	}
}

func TestEmail(t *testing.T) {
	if !local {
		// Deployed, Cloudflare sends only to an address verified in Email Routing, and only Email Routing hands the
		// Worker a message: EMAIL_TO names such an address to send to; receiving is tried on wrangler dev alone.
		to := os.Getenv("EMAIL_TO")
		if to == "" {
			t.Skip("deployed: no EMAIL_TO, an address verified in Email Routing, to send to")
		}
		post(t, "/cloud/email", url.Values{"to": {to}})
		return
	}
	post(t, "/cloud/email", url.Values{"to": {"you@example.com"}})
	if !strings.Contains(page(t, "/cloud"), "→ you@example.com") {
		t.Error("the page does not say whom the message was sent to")
	}
	// wrangler dev hands the Worker a message posted here, as Email Routing would.
	raw := "From: ada@example.com\r\nTo: site@example.com\r\nSubject: A note for the site\r\nMessage-ID: <note@example.com>\r\n\r\nHello.\r\n"
	res, err := http.Post(base+"/cdn-cgi/handler/email?from=ada@example.com&to=site@example.com", "message/rfc822", strings.NewReader(raw))
	if err != nil || res.StatusCode/100 != 2 {
		t.Fatalf("POST /cdn-cgi/handler/email: %v %v", res, err)
	}
	res.Body.Close()
	soon(t, "the received email", "ada@example.com: A note for the site")
}

// card is what the card called id shows.
func card(t *testing.T, id string) string {
	t.Helper()
	m := regexp.MustCompile(`<p id="` + id + `"[^>]*>([^<]*)</p>`).FindStringSubmatch(page(t, "/cloud"))
	if m == nil {
		t.Fatalf("the page has no card %q", id)
	}
	return m[1]
}

func TestD1ThroughWorkersGo(t *testing.T) {
	before := card(t, "rows")
	// A message posted on the home page is a row more.
	res, err := http.PostForm(base+"/messages", url.Values{"body": {"counted by D1"}})
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("POST /messages: %v %v", res, err)
	}
	res.Body.Close()
	if after := card(t, "rows"); !regexp.MustCompile(`^\d+$`).MatchString(before) || after == before {
		t.Errorf("rows: %q then %q, want a count that went up", before, after)
	}
}

func TestCacheAPI(t *testing.T) {
	card(t, "cached") // the first visit stores it
	if got := card(t, "cached"); !strings.HasPrefix(got, "hit: stored 20") {
		t.Errorf("cached: %q, want a hit", got)
	}
}

func TestDurableObjectStub(t *testing.T) {
	if got := card(t, "object"); !regexp.MustCompile(`^Hub → HTTP \d{3}$`).MatchString(got) {
		t.Errorf("object: %q, want the hub's status", got)
	}
}

func TestTCPSocket(t *testing.T) {
	if got := card(t, "socket"); !strings.Contains(got, "→ HTTP/1.") {
		t.Errorf("socket: %q, want the first line of an HTTP answer", got)
	}
}

func TestWaitUntil(t *testing.T) {
	card(t, "after") // this visit's work runs after its answer
	for start := time.Now(); time.Since(start) < 30*time.Second; time.Sleep(300 * time.Millisecond) {
		if strings.HasPrefix(card(t, "after"), "20") {
			return
		}
	}
	t.Error("after: the work after an answer never finished")
}

func TestRequestProperties(t *testing.T) {
	if got := card(t, "request"); strings.Trim(got, " ·") == "" || !strings.Contains(got, " · ") {
		t.Errorf("request: %q, want what Cloudflare knows of the request", got)
	}
}
