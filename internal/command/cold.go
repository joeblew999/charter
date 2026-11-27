package command

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/project"
)

// charter run cold asks the deployed Worker one page from many places in the world, twice, and says what a reader
// waits where no instance of the Worker is running yet: a first hit. For an app without constant traffic everywhere,
// most readers in most places are that first hit, and nothing on one machine shows it.
//
// The places are Globalping's probes (https://globalping.io, a public service: it is told the Worker's address and
// the page, nothing else). Each asks once, then again. The Worker says in the answer's x-go-runtime which runtime
// answered (app/worker/go.mjs): one that started as the instance did (warm, or new) is a first hit, one that had
// served before (reused) is not. So cold and warm are counted, not guessed. The time is from the connection made to
// the first byte: what the Worker and Cloudflare take, without the reader's distance to Cloudflare.
//
//	charter run cold                 the deployed Worker's front page, from 30 places
//	charter run cold staging         an environment
//	charter run cold -path /docs/ -from 50
//
// It is for after a change to what the Worker loads or does as it starts, never part of a deploy. Without an
// account Globalping gives an address 250 probes an hour, 50 a measurement: this takes two for each place.
func coldTask(p *project.Project, args []string) {
	env := takeEnv(p, &args)
	fs := flag.NewFlagSet("charter run cold", flag.ExitOnError)
	from := fs.Int("from", 30, "how many places to ask from (50 at most)")
	path := fs.String("path", "/", "the page to ask for")
	fs.Parse(args)
	if fs.NArg() > 0 || *from < 1 || *from > 50 || !strings.HasPrefix(*path, "/") {
		check(errors.New("usage: charter run cold [env] [-from 30] [-path /]"))
	}
	base := p.URL
	if env != "" {
		base = p.URLs[env]
	}
	at, err := url.Parse(base)
	if base == "" || err != nil || at.Scheme != "https" {
		check(errors.New(`charter.toml has no https address for it: url = "https://…" (go tool charter deploy writes it)`))
	}
	first, id, err := probed(at.Host, *path, []map[string]string{{"magic": "world"}}, *from)
	check(err)
	// The same probes again: an instance the first request started is there for the second, where Cloudflare
	// sends it to the same one.
	again, _, err := probed(at.Host, *path, id, *from)
	check(err)

	say("%s, GET %s from %d places (Globalping), the time from the connection to the first byte:", deployedName(p, env), *path, len(first))
	one, two := hitsOf(first), hitsOf(again)
	say("· the first request: %s", one)
	say("· again from each: %s", two)
	// By continent, both rounds together: what a page waits for that is far from some readers (a database's primary)
	// shows here and in no single number.
	for _, c := range continents {
		var there []probe
		for _, p := range slices.Concat(first, again) {
			if p.Probe.Continent == c.code {
				there = append(there, p)
			}
		}
		if h := hitsOf(there); len(h.fresh)+len(h.reused) > 0 {
			say("  %-14s %s", c.name+":", h)
		}
	}
	// Who answered the page's statements on D1, where the Worker says (X-D1-Served): the primary, or a copy near
	// the reader.
	if who := servedBy(slices.Concat(first, again)); who != "" {
		say("· its statements on D1 were answered by: %s", who)
	}
	if one.unknown+two.unknown > 0 {
		say("· %d answers did not say which runtime answered (x-go-runtime): a page served without the Worker, or a Worker built before charter said so", one.unknown+two.unknown)
	}
	if len(one.fresh) == 0 {
		say("· no place met a new instance: the Worker was running everywhere it was asked")
		return
	}
	half := quantile(one.fresh, 0.5)
	usually, known := measured("first-hit"+*path, half)
	if said := remark(half, usually, known, firstHitGrew, 0, millis); said != "" {
		say("· a first hit, half under %s%s", millis(half), said)
	}
}

// continents are Globalping's, in the order they are said.
var continents = []struct{ code, name string }{
	{"AF", "Africa"}, {"AS", "Asia"}, {"EU", "Europe"}, {"NA", "North America"}, {"OC", "Oceania"}, {"SA", "South America"}, {"AN", "Antarctica"},
}

// firstHitGrew is how much above what is usual here a first hit's median must be to be worth saying: it varies with
// the places Globalping gives and with the hour.
const firstHitGrew = 1.50

// hits are the answers of one round, by what answered: the first-byte times in milliseconds, sorted.
type hits struct {
	fresh, reused   []float64 // an instance that had just started; one that was running
	unknown, failed int
}

func (h hits) String() string {
	part := func(n []float64, what string) string {
		if len(n) == 0 {
			return ""
		}
		return fmt.Sprintf("%d met %s (half under %s, 90%% under %s)", len(n), what, millis(quantile(n, 0.5)), millis(quantile(n, 0.9)))
	}
	said := slices.DeleteFunc([]string{part(h.fresh, "a new instance"), part(h.reused, "a running one")}, func(s string) bool { return s == "" })
	if h.failed > 0 {
		said = append(said, fmt.Sprintf("%d got no answer", h.failed))
	}
	if len(said) == 0 {
		return "no answers"
	}
	return strings.Join(said, "; ")
}

// quantile is the value a share q of sorted is at or under.
func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[min(len(sorted)-1, int(q*float64(len(sorted))))]
}

// probe is one place's answer, as Globalping gives it (its API's MeasurementResultItem, for an http measurement).
type probe struct {
	Probe struct {
		Continent, Country, City string
	}
	Result struct {
		Status     string
		StatusCode int
		Headers    map[string]any
		Timings    struct{ FirstByte *float64 }
	}
}

// hitsOf sorts a round's answers by what the Worker said answered them.
func hitsOf(round []probe) hits {
	var h hits
	for _, p := range round {
		if p.Result.Status != "finished" || p.Result.Timings.FirstByte == nil || p.Result.StatusCode == 0 {
			h.failed++
			continue
		}
		told, _ := p.Result.Headers["x-go-runtime"].(string)
		kind, _, _ := strings.Cut(told, ";")
		switch strings.TrimSpace(kind) {
		case "warm", "new":
			h.fresh = append(h.fresh, *p.Result.Timings.FirstByte)
		case "reused":
			h.reused = append(h.reused, *p.Result.Timings.FirstByte)
		default:
			h.unknown++
		}
	}
	slices.Sort(h.fresh)
	slices.Sort(h.reused)
	return h
}

// servedBy counts who answered the statements of every answer that says: "WEUR replica 14, APAC primary 3".
func servedBy(round []probe) string {
	count := map[string]int{}
	for _, p := range round {
		told, _ := p.Result.Headers["x-d1-served"].(string)
		for _, one := range strings.Split(told, ",") {
			// "DB WEUR replica": who, without the binding's name.
			if _, who, ok := strings.Cut(strings.TrimSpace(one), " "); ok {
				count[who]++
			}
		}
	}
	names := make([]string, 0, len(count))
	for name := range count {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int { return cmp.Or(count[b]-count[a], strings.Compare(a, b)) })
	for i, name := range names {
		names[i] = fmt.Sprintf("%s %d", name, count[name])
	}
	return strings.Join(names, ", ")
}

// globalping is Globalping's API (https://api.globalping.io/v1/spec.yaml).
const globalping = "https://api.globalping.io/v1/measurements"

// probed has Globalping's probes GET https://host/path, each telling the Worker to say which runtime answered, and
// gives their answers with the measurement's id. locations is where from: places, or an earlier measurement's id
// for the same probes in the same order.
func probed(host, path string, locations any, limit int) ([]probe, string, error) {
	ask := map[string]any{
		"type": "http", "target": host, "locations": locations,
		"measurementOptions": map[string]any{"protocol": "HTTPS", "request": map[string]any{
			"method": "GET", "path": path, "headers": map[string]string{"x-go-runtime": "?"}}},
	}
	// An earlier measurement's probes are all of them: Globalping takes no limit beside its id.
	if _, same := locations.(string); !same {
		ask["limit"] = limit
	}
	body, _ := json.Marshal(ask)
	client := &http.Client{Timeout: 30 * time.Second}
	call := func(method, address string, body io.Reader) (*http.Response, error) {
		req, err := http.NewRequest(method, address, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "charter-cli")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return client.Do(req)
	}
	res, err := call(http.MethodPost, globalping, bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("Globalping: %w", err)
	}
	said, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return nil, "", fmt.Errorf("Globalping gives this address no more probes for now (250 an hour without an account): again in %s seconds, or with a smaller -from", cmpOr(res.Header.Get("Retry-After"), "some"))
	}
	var made struct{ ID string }
	if res.StatusCode != http.StatusAccepted || json.Unmarshal(said, &made) != nil || made.ID == "" {
		return nil, "", fmt.Errorf("Globalping: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(said)))
	}
	// Its answer is asked for until it is no longer in progress, half a second after each answer: Globalping's own
	// guideline, and its limit of two requests a second for a measurement.
	for waited := time.Duration(0); waited < 90*time.Second; waited += 500 * time.Millisecond {
		time.Sleep(500 * time.Millisecond)
		res, err := call(http.MethodGet, globalping+"/"+made.ID, nil)
		if err != nil {
			return nil, "", fmt.Errorf("Globalping: %w", err)
		}
		var got struct {
			Status  string
			Results []probe
		}
		err = json.NewDecoder(res.Body).Decode(&got)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("Globalping: HTTP %d reading the measurement", res.StatusCode)
		}
		if got.Status != "in-progress" {
			return got.Results, made.ID, nil
		}
	}
	return nil, "", errors.New("Globalping: the measurement did not finish in 90 seconds")
}
