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
