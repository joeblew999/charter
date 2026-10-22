package web

import "net/http"

// ServeMux is what NewServeMux returns: net/http's ServeMux API, with Go 1.22 patterns ("GET /items/{id}", {$},
// {rest...}) under both compilers.
type ServeMux interface {
	http.Handler
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}
