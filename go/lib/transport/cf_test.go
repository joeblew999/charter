package transport

import (
	"net/http/httptest"
	"testing"
)

func TestCFComesFromGoMjsAndLeavesTheHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(cfHeader, `{"country":"DE","city":"Munich","timezone":"Europe/Berlin","asn":3320,"colo":"MUC","extra":true}`)
	r = withCF(r)
	cf := CFOf(r)
	if cf.Country != "DE" || cf.City != "Munich" || cf.Timezone != "Europe/Berlin" || cf.ASN != 3320 || cf.Colo != "MUC" {
		t.Errorf("CF = %+v", cf)
	}
	if r.Header.Get(cfHeader) != "" {
		t.Error("the handler sees the header")
	}
}

func TestNoOrBadCFIsTheZeroCF(t *testing.T) {
	for _, raw := range []string{"", "not json"} {
		r := httptest.NewRequest("GET", "/", nil)
		if raw != "" {
			r.Header.Set(cfHeader, raw)
		}
		if cf := CFOf(withCF(r)); cf != (CF{}) {
			t.Errorf("%q: CF = %+v, want zero", raw, cf)
		}
	}
	if cf := CFOf(httptest.NewRequest("GET", "/", nil)); cf != (CF{}) {
		t.Errorf("a request withCF never saw: %+v", cf)
	}
}

// TestCFAsCloudflareSendsItNow: the object a preview got from Cloudflare on 2026-10-22, isEUCountry a boolean among
// fields CF doesn't have. Every field CF has is read, and the object is kept as it came.
func TestCFAsCloudflareSendsItNow(t *testing.T) {
	raw := `{"httpProtocol":"HTTP/2","clientTcpRtt":4,"requestHeaderNames":[":method","accept"],"colo":"BKK","asn":133481,` +
		`"asOrganization":"AIS Fibre","country":"TH","isEUCountry":false,"city":"Rayong","continent":"AS","region":"Rayong",` +
		`"regionCode":"21","timezone":"Asia/Bangkok","longitude":"101.25798","latitude":"12.68095","postalCode":"21000",` +
		`"tlsClientAuth":{"certPresented":"0"}}`
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(cfHeader, raw)
	r = withCF(r)
	want := CF{Country: "TH", Region: "Rayong", RegionCode: "21", City: "Rayong", PostalCode: "21000", Continent: "AS",
		Timezone: "Asia/Bangkok", Latitude: "12.68095", Longitude: "101.25798", Colo: "BKK", ASN: 133481}
	if got := CFOf(r); got != want {
		t.Errorf("CF = %+v\nwant %+v", got, want)
	}
	if string(CFJSON(r)) != raw {
		t.Errorf("CFJSON = %s", CFJSON(r))
	}
}

// TestCFFieldsOneByOne: a field sent some other way costs only itself; isEUCountry is "1" for "1" or true.
func TestCFFieldsOneByOne(t *testing.T) {
	for raw, want := range map[string]CF{
		`{"city":42,"timezone":"Europe/Berlin","country":"DE"}`:         {City: "42", Timezone: "Europe/Berlin", Country: "DE"},
		`{"city":{"name":"Munich"},"timezone":"Europe/Berlin"}`:         {Timezone: "Europe/Berlin"},
		`{"isEUCountry":true,"country":"DE"}`:                           {IsEUCountry: "1", Country: "DE"},
		`{"isEUCountry":"1","country":"FR"}`:                            {IsEUCountry: "1", Country: "FR"},
		`{"isEUCountry":false,"country":"TH"}`:                          {Country: "TH"},
		`{"asn":"3320","latitude":48.13743,"timezone":"Europe/Berlin"}`: {ASN: 3320, Latitude: "48.13743", Timezone: "Europe/Berlin"},
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set(cfHeader, raw)
		if got := CFOf(withCF(r)); got != want {
			t.Errorf("%s: CF = %+v, want %+v", raw, got, want)
		}
	}
}
