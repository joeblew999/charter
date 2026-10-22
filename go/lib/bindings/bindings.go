// Package bindings is a Go Worker's Cloudflare bindings for app code, called from plain net/http handlers, each with
// what stands in for it off Workers (go test, an ordinary Go server) built in, so app code needs no file per platform.
// What stands in follows charter.toml (package project): the same limits, the same migrations.
//
//	bindings.Var("APP_ENV")                 a Worker variable; off Workers, the process environment
//	bindings.Allow("WRITES", key)           a Rate Limiting binding; off Workers, charter.toml's limit, counted in memory
//	bindings.StaticAssets("dist/site")      Static Assets; off Workers, the folder, from disk
//	d1.Open("DB")                           a D1 database (package d1); off Workers, SQLite in memory, migrated
//	hub.DurableObject[T]("ROOM", topic)     publishing to a Durable Object (package hub); off Workers, in memory
//	transport.CFOf(r)                       request.cf; off Workers, the zero CF (transport.WithCF in a test)
package bindings

import (
	"errors"
	"net/http"
)

// ErrNoBinding means the binding isn't configured: no such binding on the Worker, or off Workers none in
// charter.toml. Allow still allows (it fails open, so a missing binding never takes an app down): log it.
var ErrNoBinding = errors.New("bindings: binding not configured")

// ClientKey is a request's caller for a per-client limit: the client's IP, which Cloudflare puts in CF-Connecting-IP,
// or "local" without one (local workerd, go test).
func ClientKey(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	return "local"
}
