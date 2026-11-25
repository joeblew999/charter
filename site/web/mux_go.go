package web

import "net/http"

// NewServeMux is http.NewServeMux.
func NewServeMux() ServeMux { return http.NewServeMux() }
