//go:build js && wasm

package auth

import (
	"net/http"

	"github.com/syumai/workers-go/cloudflare/fetch"
)

// client fetches issuers' keys: on Workers, through the Worker's fetch.
func client() *http.Client { return fetch.NewClient().HTTPClient(fetch.RedirectModeFollow) }
