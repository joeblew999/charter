package transport

import (
	"context"
	"encoding/json"
	"net/http"
)

// CF is what Cloudflare knows of where a request came from: its request.cf, the fields an app
// uses. On Cloudflare, worker/go.mjs passes it with every request (and with a WebSocket upgrade);
// natively, and for a request that did not come through go.mjs, it is the zero CF.
//
// workers-go's fetch.NewIncomingProperties reads the same from workers-go's own request path,
// which Run does not use (it panics there: "trigger object was not found"). Use this instead.
type CF struct {
	Country     string `json:"country"`     // ISO 3166-1 alpha-2, e.g. "DE"
	Region      string `json:"region"`      // e.g. "Bavaria"
	RegionCode  string `json:"regionCode"`  // ISO 3166-2, e.g. "BY"
	City        string `json:"city"`        // e.g. "Munich"
	PostalCode  string `json:"postalCode"`  //
	Continent   string `json:"continent"`   // e.g. "EU"
	Timezone    string `json:"timezone"`    // IANA, e.g. "Europe/Berlin"
	Latitude    string `json:"latitude"`    //
	Longitude   string `json:"longitude"`   //
	Colo        string `json:"colo"`        // the Cloudflare data center, e.g. "MUC"
	ASN         int    `json:"asn"`         // the client's network
	IsEUCountry string `json:"isEUCountry"` // "1" in the EU
}

// cfHeader carries request.cf from worker/go.mjs to answer as one line of the request's head.
// go.mjs drops a header of that name the client sent, and answer removes it from the request, so
// a handler never sees it and a client cannot set CF.
const cfHeader = "X-Charter-Cf"

type cfKey struct{}

// CFOf is r's CF.
func CFOf(r *http.Request) CF {
	cf, _ := r.Context().Value(cfKey{}).(CF)
	return cf
}

// withCF moves cfHeader's JSON out of r's headers into its context. A missing or bad value leaves
// the zero CF.
func withCF(r *http.Request) *http.Request {
	raw := r.Header.Get(cfHeader)
	r.Header.Del(cfHeader)
	var cf CF
	if raw == "" || json.Unmarshal([]byte(raw), &cf) != nil {
		return r
	}
	return r.WithContext(context.WithValue(r.Context(), cfKey{}, cf))
}
