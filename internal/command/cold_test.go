package command

import (
	"encoding/json"
	"strings"
	"testing"
)

// A round's answers are counted by what the Worker said answered them: a runtime that started with the instance is
// a first hit, one that had served is not, and an answer that says nothing or never came is counted apart.
func TestHitsOf(t *testing.T) {
	// As Globalping's API gives a measurement's results (its MeasurementResultItem).
	const results = `[
		{"probe": {"continent": "EU", "country": "DE", "city": "Falkenstein"},
		 "result": {"status": "finished", "statusCode": 200, "headers": {"x-go-runtime": "warm; 1 runtimes, 18 MB of 80"}, "timings": {"firstByte": 640}}},
		{"probe": {"continent": "AS", "country": "JP", "city": "Tokyo"},
		 "result": {"status": "finished", "statusCode": 200, "headers": {"x-go-runtime": "new; 2 runtimes, 33 MB of 80"}, "timings": {"firstByte": 910}}},
		{"probe": {"continent": "NA", "country": "US", "city": "Dallas"},
		 "result": {"status": "finished", "statusCode": 200, "headers": {"x-go-runtime": "reused; 1 runtimes, 19 MB of 80"}, "timings": {"firstByte": 95}}},
		{"probe": {"continent": "NA", "country": "US", "city": "Ashburn"},
		 "result": {"status": "finished", "statusCode": 200, "headers": {"content-type": "text/html"}, "timings": {"firstByte": 20}}},
		{"probe": {"continent": "SA", "country": "BR", "city": "Sao Paulo"},
		 "result": {"status": "failed", "rawOutput": "timeout"}}
	]`
	var round []probe
	if err := json.Unmarshal([]byte(results), &round); err != nil {
		t.Fatal(err)
	}
	h := hitsOf(round)
	if len(h.fresh) != 2 || h.fresh[0] != 640 || h.fresh[1] != 910 {
		t.Errorf("new instances: %v, want [640 910]", h.fresh)
	}
	if len(h.reused) != 1 || h.reused[0] != 95 {
		t.Errorf("running instances: %v, want [95]", h.reused)
	}
	if h.unknown != 1 || h.failed != 1 {
		t.Errorf("unknown %d, failed %d: want 1 and 1", h.unknown, h.failed)
	}
	if got, want := h.String(), "2 met a new instance (half under 910 ms, 90% under 910 ms); 1 met a running one (half under 95 ms, 90% under 95 ms); 1 got no answer"; got != want {
		t.Errorf("said %q, want %q", got, want)
	}
	if got := (hits{}).String(); got != "no answers" {
		t.Errorf("an empty round said %q", got)
	}
}

// Who answered a page's statements on D1 is counted from what each answer says, the most first.
func TestServedBy(t *testing.T) {
	const results = `[
		{"result": {"headers": {"x-d1-served": "DB WEUR replica, DB WEUR replica"}}},
		{"result": {"headers": {"x-d1-served": "DB APAC primary"}}},
		{"result": {"headers": {"x-d1-served": "DB WEUR replica"}}},
		{"result": {"headers": {"content-type": "text/html"}}}
	]`
	var round []probe
	if err := json.Unmarshal([]byte(results), &round); err != nil {
		t.Fatal(err)
	}
	if got, want := servedBy(round), "WEUR replica 3, APAC primary 1"; got != want {
		t.Errorf("servedBy: %q, want %q", got, want)
	}
	if got := servedBy(nil); got != "" {
		t.Errorf("of nothing: %q", got)
	}
}

func TestQuantile(t *testing.T) {
	ten := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for q, want := range map[float64]float64{0.5: 6, 0.9: 10, 0: 1, 1: 10} {
		if got := quantile(ten, q); got != want {
			t.Errorf("quantile(%v) = %v, want %v", q, got, want)
		}
	}
	if got := quantile(nil, 0.5); got != 0 {
		t.Errorf("of nothing: %v", got)
	}
}

// The countries to ask from, as they are typed.
func TestCountriesIn(t *testing.T) {
	for typed, want := range map[string]string{"DE,US": "DE,US", " de , us,DE ": "DE,US", "": "", ",": "", "th": "TH"} {
		if got := strings.Join(countriesIn(typed), ","); got != want {
			t.Errorf("countriesIn(%q) = %q, want %q", typed, got, want)
		}
	}
}
