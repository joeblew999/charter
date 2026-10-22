//go:build !tinygo

package web

import "net/http"

// NewServeMux is http.NewServeMux. Under TinyGo it is a copy of Go 1.26's (mux_tinygo.go), since TinyGo's has no
// Go 1.22 patterns.
func NewServeMux() ServeMux { return http.NewServeMux() }
