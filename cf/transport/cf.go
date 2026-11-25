package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
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
	IsEUCountry string `json:"isEUCountry"` // "1" in the EU (Cloudflare sends "1" or true), else ""
}

// cfHeader carries request.cf from worker/go.mjs to answer as one line of the request's head.
// go.mjs drops a header of that name the client sent, and answer removes it from the request, so
// a handler never sees it and a client cannot set CF.
const cfHeader = "X-Charter-Cf"

type cfKey struct{}

// cfValue is what withCF keeps: the fields read, and the object as it came.
type cfValue struct {
	cf  CF
	raw json.RawMessage
}

// CFOf is r's CF.
func CFOf(r *http.Request) CF {
	v, _ := r.Context().Value(cfKey{}).(cfValue)
	return v.cf
}

// WithCF is r carrying cf as its request.cf: for a test, or a server off Cloudflare that knows where its requests come
// from. On Cloudflare, worker/go.mjs passes the real one.
func WithCF(r *http.Request, cf CF) *http.Request {
	raw, _ := json.Marshal(cf)
	return r.WithContext(context.WithValue(r.Context(), cfKey{}, cfValue{cf, raw}))
}

// CFJSON is r's request.cf as worker/go.mjs passed it (CF's fields, exactly as Cloudflare sent them), nil without
// one: for an app that reads a field its own way.
func CFJSON(r *http.Request) json.RawMessage {
	v, _ := r.Context().Value(cfKey{}).(cfValue)
	return v.raw
}

// withCF moves cfHeader's JSON out of r's headers into its context. Each field is read on its own: Cloudflare changes
// a field's type now and then (isEUCountry was "1" or absent, and is now true or false), and a field it sends some
// other way must cost that field only, never the rest. A missing value, or one that isn't a JSON object, leaves the
// zero CF.
func withCF(r *http.Request) *http.Request {
	raw := r.Header.Get(cfHeader)
	r.Header.Del(cfHeader)
	var fields map[string]json.RawMessage
	if raw == "" || json.Unmarshal([]byte(raw), &fields) != nil {
		return r
	}
	cf := CF{
		Country: text(fields["country"]), Region: text(fields["region"]), RegionCode: text(fields["regionCode"]),
		City: text(fields["city"]), PostalCode: text(fields["postalCode"]), Continent: text(fields["continent"]),
		Timezone: text(fields["timezone"]), Latitude: text(fields["latitude"]), Longitude: text(fields["longitude"]),
		Colo: text(fields["colo"]),
	}
	cf.ASN, _ = strconv.Atoi(text(fields["asn"]))
	if eu := text(fields["isEUCountry"]); eu == "1" || eu == "true" {
		cf.IsEUCountry = "1"
	}
	return r.WithContext(context.WithValue(r.Context(), cfKey{}, cfValue{cf, json.RawMessage(raw)}))
}

// text is a JSON value as text: a string's contents, a number or a boolean as written, "" for anything else.
func text(v json.RawMessage) string {
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	if t := strings.TrimSpace(string(v)); t == "true" || t == "false" || t != "" && strings.Trim(t, "-+.eE0123456789") == "" {
		return t
	}
	return ""
}
