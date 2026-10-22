//go:build tinygo

package web

import "github.com/joeblew999/charter/go/lib/web/internal/servemux"

// NewServeMux is Go 1.26's ServeMux (internal/servemux): TinyGo's net/http still routes like Go 1.21's, so
// "GET /items/{id}", {$} and {rest...} never match there (tinygo-org/net#55, fixed by tinygo-org/net#56 once merged).
// When a TinyGo release has it, delete this file and internal/servemux, and drop mux_go.go's build constraint.
func NewServeMux() ServeMux { return servemux.NewServeMux() }
