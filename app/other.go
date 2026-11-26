//go:build !(js && wasm)

package app

import (
	"log"
	"net/http"
)

// Run is for a Worker. A program built for this machine says so and stops: nothing of charter runs off Cloudflare.
func Run(http.Handler) {
	log.Fatal("this is a Cloudflare Worker: go tool charter dev runs it here, go tool charter deploy on Cloudflare")
}

// Serve is h: what it adds on Cloudflare has nothing to add here.
func Serve(h http.Handler) http.Handler { return h }

// besideTheWasm is empty off a Worker: there is nothing beside a Wasm that is not there.
func besideTheWasm(string) string { return "" }
