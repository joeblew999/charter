//go:build !(js && wasm)

package auth

import "net/http"

// client fetches issuers' keys.
func client() *http.Client { return http.DefaultClient }
