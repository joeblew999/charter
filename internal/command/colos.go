package command

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/project"
)

// The map: which of Cloudflare's data centres Globalping's probes reach. Cloudflare runs a Worker in the data
// centre a request arrives at, and which one that is depends on the network the request comes from, not on a place
// on a map: two probes a city apart can reach different ones. So it is found by asking: one probe in each country
// Globalping has probes in asks the Worker for a page, and the answer's cf-ray header ends with the code of the
// data centre that answered (BKK, FRA). What each probe reached is a sighting, and the sightings are the map.
//
// The way of it is cloudflare-ping's (github.com/joeblew999/cloudflare-ping, src/map.ts and src/globalping.ts),
// written again here so that charter does it itself, with no other service than the two that own the lists:
//
//   - the places are Globalping's own list of its probes (api.globalping.io/v1/probes, which costs no test);
//   - a test asks from many places at once, one probe each, 50 places at most without an account;
//   - what a data centre's code means is Cloudflare's own list of them (www.cloudflarestatus.com).
//
// charter run cold -everywhere makes the map and says, for each data centre reached, what a reader there waited. It
// is kept in .charter/colos.json. It costs one of Globalping's tests for each country: its allowance for an hour
// without an account is 250, and GLOBALPING_TOKEN, when it is set, is the account that gives more.

const (
	globalpingProbes = "https://api.globalping.io/v1/probes"
	globalpingLimits = "https://api.globalping.io/v1/limits"
	// Cloudflare's status page lists every data centre as a component, under its region.
	cloudflareDatacentres = "https://www.cloudflarestatus.com/api/v2/components.json"
	// Places in one test: what Globalping takes without an account.
	placesPerTest = 50
)

// place is where a test is asked from: a country, by its ISO code. One probe there.
type place struct {
	Country string `json:"country"`
	Limit   int    `json:"limit"`
}

// sighting is one probe's answer: where the probe was, as Globalping says, and the data centre that answered it.
// No data centre when the probe did not reach the Worker.
type sighting struct {
	Country   string  `json:"country"`
	City      string  `json:"city"`
	ASN       int     `json:"asn"`
	Network   string  `json:"network"`
	Colo      string  `json:"colo,omitempty"`
	Fresh     bool    `json:"fresh"`     // a new instance answered: a first hit
	FirstByte float64 `json:"firstByte"` // ms from the connection to the first byte
	// Served is who answered the page's statements on D1, as the Worker says: "DB WEUR replica".
	Served string `json:"served,omitempty"`
}

// datacentre is one of Cloudflare's, as its own list names it: BKK, "Bangkok, Thailand", "Asia".
type datacentre struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Region string `json:"region,omitempty"`
}

// colos is the map as it is kept in .charter/colos.json.
type colos struct {
	At          time.Time    `json:"at"`
	Worker      string       `json:"worker"`
	Path        string       `json:"path"`
	Countries   int          `json:"countries"`   // asked
	Datacentres []datacentre `json:"datacentres"` // Cloudflare's list, as reference
	Sightings   []sighting   `json:"sightings"`
}

// everywhere makes the map: one probe in every country Globalping has probes in asks the Worker at host for path,
// and what each reached is said and kept.
func everywhere(p *project.Project, worker, host, path string) error {
	countries, err := probeCountries()
	if err != nil {
		return err
	}
	if left, reset, err := testsLeft(); err == nil && left < len(countries) {
		return fmt.Errorf("Globalping has %d tests left for this hour and the map takes %d, one for each country with a probe: again in %d minutes, or with GLOBALPING_TOKEN set to an account's token", left, len(countries), reset/60+1)
	}
	centres, err := datacentres()
	if err != nil {
		return err
	}
	var seen []sighting
	for chunk := range slices.Chunk(countries, placesPerTest) {
		places := make([]place, len(chunk))
		for i, country := range chunk {
			places[i] = place{country, 1}
		}
		round, _, err := probed(host, path, places, 0)
		if err != nil {
			return err
		}
		seen = append(seen, sightingsOf(round)...)
	}
	m := colos{At: time.Now().UTC(), Worker: worker, Path: path, Countries: len(countries), Datacentres: centres, Sightings: seen}
	if err := os.MkdirAll(filepath.Join(p.Root, ".charter"), 0o755); err != nil {
		return err
	}
	kept, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(p.Root, ".charter", "colos.json"), kept, 0o644); err != nil {
		return err
	}
	for _, line := range m.said() {
		say("%s", line)
	}
	if who := servedBy(slices.Concat(roundsOf(seen)...)); who != "" {
		say("its statements on D1 were answered by: %s", who)
	}
	say("kept in .charter/colos.json")
	return nil
}

// said is the map in words: how far it got, then each data centre reached, the most places first.
func (m colos) said() []string {
	names := map[string]datacentre{}
	for _, d := range m.Datacentres {
		names[d.Code] = d
	}
	type reached struct {
		countries []string
		fresh     []float64
		running   []float64
	}
	by := map[string]*reached{}
	answered := 0
	for _, s := range m.Sightings {
		if s.Colo == "" {
			continue
		}
		answered++
		r := by[s.Colo]
		if r == nil {
			r = &reached{}
			by[s.Colo] = r
		}
		if !slices.Contains(r.countries, s.Country) {
			r.countries = append(r.countries, s.Country)
		}
		if s.Fresh {
			r.fresh = append(r.fresh, s.FirstByte)
		} else {
			r.running = append(r.running, s.FirstByte)
		}
	}
	codes := make([]string, 0, len(by))
	for code := range by {
		codes = append(codes, code)
	}
	slices.SortFunc(codes, func(a, b string) int {
		return cmp.Or(len(by[b].countries)-len(by[a].countries), strings.Compare(a, b))
	})
	lines := []string{
		fmt.Sprintf("%s, GET %s, one probe in each of %d countries (Globalping): %d answered, and reached %d of the %d data centres Cloudflare lists",
			m.Worker, m.Path, m.Countries, answered, len(by), len(m.Datacentres)),
	}
	for _, code := range codes {
		r := by[code]
		slices.Sort(r.countries)
		slices.Sort(r.fresh)
		slices.Sort(r.running)
		var waited []string
		if len(r.fresh) > 0 {
			waited = append(waited, fmt.Sprintf("a new instance %s", millis(quantile(r.fresh, 0.5))))
		}
		if len(r.running) > 0 {
			waited = append(waited, fmt.Sprintf("a running one %s", millis(quantile(r.running, 0.5))))
		}
		lines = append(lines, fmt.Sprintf("  %s  %-34s from %-18s %s", code, cmp.Or(names[code].Name, "not in Cloudflare's list"),
			strings.Join(r.countries, ","), strings.Join(waited, ", ")))
	}
	return lines
}

// sightingsOf reads a test's answers as sightings: the probe's place, and the data centre its answer's cf-ray names.
func sightingsOf(round []probe) []sighting {
	seen := make([]sighting, 0, len(round))
	for _, p := range round {
		s := sighting{Country: p.Probe.Country, City: p.Probe.City, ASN: p.Probe.ASN, Network: p.Probe.Network}
		if p.Result.Status == "finished" && p.Result.Timings.FirstByte != nil {
			ray, _ := p.Result.Headers["cf-ray"].(string)
			s.Colo = coloOf(ray)
			s.FirstByte = *p.Result.Timings.FirstByte
			told, _ := p.Result.Headers["x-go-runtime"].(string)
			kind, _, _ := strings.Cut(told, ";")
			s.Fresh = kind == "warm" || kind == "new"
			s.Served, _ = p.Result.Headers["x-d1-served"].(string)
		}
		seen = append(seen, s)
	}
	return seen
}

// A cf-ray is an id, a dash, and the data centre's code: 8f2c1a9d4e7b6a53-BKK.
var rayColo = regexp.MustCompile(`-([A-Z]{3})$`)

// coloOf is the data centre a cf-ray header names, "" when it names none.
func coloOf(ray string) string {
	if m := rayColo.FindStringSubmatch(strings.TrimSpace(ray)); m != nil {
		return m[1]
	}
	return ""
}

// listed is a GET of one of the lists, with Globalping's token when there is one.
func listed(address string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "charter-cli")
	if token := os.Getenv("GLOBALPING_TOKEN"); token != "" && strings.HasPrefix(address, "https://api.globalping.io/") {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", address, res.StatusCode)
	}
	return body, nil
}

// probeCountries is every country Globalping has a probe in now, by its ISO code, in order.
func probeCountries() ([]string, error) {
	body, err := listed(globalpingProbes)
	if err != nil {
		return nil, err
	}
	return countriesOf(body)
}

// countriesOf reads Globalping's list of probes for the countries they are in.
func countriesOf(body []byte) ([]string, error) {
	var probes []struct {
		Location struct{ Country string }
	}
	if err := json.Unmarshal(body, &probes); err != nil {
		return nil, fmt.Errorf("Globalping's list of probes: %w", err)
	}
	var countries []string
	for _, p := range probes {
		if c := p.Location.Country; c != "" && !slices.Contains(countries, c) {
			countries = append(countries, c)
		}
	}
	if len(countries) == 0 {
		return nil, errors.New("Globalping's list of probes has none")
	}
	slices.Sort(countries)
	return countries, nil
}

// testsLeft is what Globalping has left of this hour's tests for whoever asks, and in how many seconds there are more.
func testsLeft() (left, reset int, err error) {
	body, err := listed(globalpingLimits)
	if err != nil {
		return 0, 0, err
	}
	var limits struct {
		RateLimit struct {
			Measurements struct {
				Create struct{ Remaining, Reset int }
			}
		}
	}
	if err := json.Unmarshal(body, &limits); err != nil {
		return 0, 0, err
	}
	return limits.RateLimit.Measurements.Create.Remaining, limits.RateLimit.Measurements.Create.Reset, nil
}

// datacentres is Cloudflare's own list of its data centres.
func datacentres() ([]datacentre, error) {
	body, err := listed(cloudflareDatacentres)
	if err != nil {
		return nil, err
	}
	return datacentresOf(body)
}

// A data centre is a component whose name ends with its code in brackets after a dash: "Bangkok, Thailand - (BKK)",
// "Hong Kong - (HKG)". A service may carry a code in brackets too ("Data Loss Prevention (DLP)"), without the dash.
var datacentreName = regexp.MustCompile(`^(.*)\s-\s\(([A-Z]{3})\)\s*$`)

// datacentresOf reads Cloudflare's status page's components for its data centres, each under the region that is
// its group.
func datacentresOf(body []byte) ([]datacentre, error) {
	var page struct {
		Components []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Group   bool   `json:"group"`
			GroupID string `json:"group_id"`
		}
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("Cloudflare's list of data centres: %w", err)
	}
	regions := map[string]string{}
	for _, c := range page.Components {
		if c.Group {
			regions[c.ID] = c.Name
		}
	}
	var centres []datacentre
	for _, c := range page.Components {
		if m := datacentreName.FindStringSubmatch(c.Name); m != nil {
			centres = append(centres, datacentre{Code: m[2], Name: strings.TrimSpace(m[1]), Region: regions[c.GroupID]})
		}
	}
	if len(centres) == 0 {
		return nil, errors.New("Cloudflare's list has no data centres in it")
	}
	slices.SortFunc(centres, func(a, b datacentre) int { return strings.Compare(a.Code, b.Code) })
	return centres, nil
}

// roundsOf is the sightings as answers that say who served them, for servedBy.
func roundsOf(seen []sighting) [][]probe {
	round := make([]probe, len(seen))
	for i, s := range seen {
		round[i].Result.Headers = map[string]any{"x-d1-served": s.Served}
	}
	return [][]probe{round}
}
