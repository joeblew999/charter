package command

import (
	"encoding/json"
	"strings"
	"testing"
)

// Cloudflare's status page lists a data centre as a component named for its city with its code after a dash, under
// the group that is its region; a service with a code in brackets and no dash is not one.
func TestDatacentresOf(t *testing.T) {
	const page = `{"components": [
		{"id": "g1", "name": "Asia", "group": true},
		{"id": "g2", "name": "Cloudflare Sites and Services", "group": true},
		{"id": "c1", "name": "Bangkok, Thailand - (BKK)", "group_id": "g1"},
		{"id": "c2", "name": "Hong Kong - (HKG)", "group_id": "g1"},
		{"id": "c3", "name": "Data Loss Prevention (DLP)", "group_id": "g2"},
		{"id": "c4", "name": "Workers", "group_id": "g2"}
	]}`
	got, err := datacentresOf([]byte(page))
	if err != nil {
		t.Fatal(err)
	}
	want := []datacentre{{"BKK", "Bangkok, Thailand", "Asia"}, {"HKG", "Hong Kong", "Asia"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("data centres: %+v, want %+v", got, want)
	}
	if _, err := datacentresOf([]byte(`{"components": []}`)); err == nil {
		t.Error("a list with no data centres was taken")
	}
}

// The places are the countries Globalping has a probe in, each once, in order.
func TestCountriesOf(t *testing.T) {
	const probes = `[{"location": {"country": "TH", "city": "Bangkok"}}, {"location": {"country": "DE"}},
		{"location": {"country": "TH", "city": "Chiang Mai"}}, {"location": {}}]`
	got, err := countriesOf([]byte(probes))
	if err != nil || strings.Join(got, ",") != "DE,TH" {
		t.Errorf("countries: %v, %v", got, err)
	}
	if _, err := countriesOf([]byte(`[]`)); err == nil {
		t.Error("no probes was taken for a list")
	}
}

// The data centre that answered is the code a cf-ray ends with.
func TestColoOf(t *testing.T) {
	for ray, want := range map[string]string{"8f2c1a9d4e7b6a53-BKK": "BKK", " 8f2c1a9d4e7b6a53-FRA ": "FRA", "8f2c1a9d4e7b6a53": "", "": "", "x-bkk": ""} {
		if got := coloOf(ray); got != want {
			t.Errorf("coloOf(%q) = %q, want %q", ray, got, want)
		}
	}
}

// A test's answers are sightings, and the sightings are the map: each data centre reached, with the countries that
// reached it, the most first; a probe that reached nothing counts for none.
func TestTheMap(t *testing.T) {
	// As Globalping's API gives a measurement's results.
	const results = `[
		{"probe": {"country": "TH", "city": "Bangkok", "asn": 7470, "network": "True Internet"},
		 "result": {"status": "finished", "statusCode": 200, "timings": {"firstByte": 640},
		            "headers": {"cf-ray": "8f2c1a9d4e7b6a53-BKK", "x-go-runtime": "warm; 1 runtimes, 18 MB of 80", "x-d1-served": "DB APAC replica"}}},
		{"probe": {"country": "DE", "city": "Falkenstein", "asn": 24940, "network": "Hetzner"},
		 "result": {"status": "finished", "statusCode": 200, "timings": {"firstByte": 55},
		            "headers": {"cf-ray": "8f2c1a9d4e7b6a54-FRA", "x-go-runtime": "reused; 1 runtimes, 19 MB of 80", "x-d1-served": "DB WEUR replica"}}},
		{"probe": {"country": "AT", "city": "Vienna", "asn": 1, "network": "A1"},
		 "result": {"status": "finished", "statusCode": 200, "timings": {"firstByte": 700},
		            "headers": {"cf-ray": "8f2c1a9d4e7b6a55-FRA", "x-go-runtime": "new; 2 runtimes, 33 MB of 80"}}},
		{"probe": {"country": "BR", "city": "Sao Paulo", "asn": 2, "network": "Vivo"},
		 "result": {"status": "failed"}}
	]`
	var round []probe
	if err := json.Unmarshal([]byte(results), &round); err != nil {
		t.Fatal(err)
	}
	seen := sightingsOf(round)
	if len(seen) != 4 || seen[0] != (sighting{"TH", "Bangkok", 7470, "True Internet", "BKK", true, 640, "DB APAC replica"}) || seen[3].Colo != "" {
		t.Fatalf("sightings: %+v", seen)
	}
	m := colos{Worker: "my-app", Path: "/", Countries: 4, Sightings: seen,
		Datacentres: []datacentre{{"BKK", "Bangkok, Thailand", "Asia"}, {"FRA", "Frankfurt, Germany", "Europe"}, {"SIN", "Singapore", "Asia"}}}
	said := m.said()
	if len(said) != 3 {
		t.Fatalf("said: %q", said)
	}
	if want := "my-app, GET /, one probe in each of 4 countries (Globalping): 3 answered, and reached 2 of the 3 data centres Cloudflare lists"; said[0] != want {
		t.Errorf("said %q, want %q", said[0], want)
	}
	// Frankfurt first: two countries reached it.
	for i, want := range [][]string{{"FRA", "Frankfurt, Germany", "AT,DE", "a new instance 700 ms", "a running one 55 ms"}, {"BKK", "Bangkok, Thailand", "TH", "a new instance 640 ms"}} {
		for _, part := range want {
			if !strings.Contains(said[i+1], part) {
				t.Errorf("line %d %q has no %q", i+1, said[i+1], part)
			}
		}
	}
	if got := servedBy(roundsOf(seen)[0]); got != "APAC replica 1, WEUR replica 1" {
		t.Errorf("served by: %q", got)
	}
}
