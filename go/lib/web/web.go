// Package web is net/http helpers for a Go Worker's pages that work the same under TinyGo on Cloudflare Workers and
// standard Go:
//
//   - TinyGo's net/http has the ServeMux from before Go 1.22: no "GET /path" patterns and no {$}. Register plain
//     paths and check the method in the handler with Allow.
//   - A response workers-go streams arrives chunked, which broke htmx's history restore under workerd: Render
//     buffers a page and sends it with a Content-Length.
//   - A Worker's request URL is absolute, a Go server's isn't: Origin is the scheme and host either way.
package web

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Allow reports whether r uses one of methods (GET also allows HEAD). Otherwise it replies 405 with an Allow header
// and returns false.
func Allow(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	if slices.Contains(methods, r.Method) || (r.Method == http.MethodHead && slices.Contains(methods, http.MethodGet)) {
		return true
	}
	w.Header().Set("Allow", strings.Join(methods, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

// Page is what Render renders: a gsx node, a templ component, anything that writes itself.
type Page interface {
	Render(ctx context.Context, w io.Writer) error
}

// pages are the buffers pages render into, kept between requests. A page of 50–170 KB grown from an empty buffer
// copies itself over and over (a home page allocated 560 KB, mostly that).
var pages = sync.Pool{New: func() any { return bytes.NewBuffer(make([]byte, 0, 256<<10)) }}

// Render renders page into a buffer and writes it as HTML with a Content-Length. If rendering fails it replies 500
// instead and returns the error, so a half-rendered page is never sent.
func Render(w http.ResponseWriter, r *http.Request, page Page) error {
	buf := pages.Get().(*bytes.Buffer)
	buf.Reset()
	defer pages.Put(buf)
	if err := page.Render(r.Context(), buf); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return err
	}
	WriteHTML(w, buf.Bytes())
	return nil
}

// WriteHTML writes b as text/html with a Content-Length.
func WriteHTML(w http.ResponseWriter, b []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.Write(b)
}

// GetOnly wraps h so it answers only GET and HEAD (an http.FileServer answers any method).
func GetOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if Allow(w, r, http.MethodGet) {
			h.ServeHTTP(w, r)
		}
	})
}

// Origin is the scheme and host the request was made to ("https://app.example.workers.dev",
// "http://localhost:8787"), for absolute URLs without configuring one. On Workers the request URL is absolute; under
// standard Go the scheme comes from TLS and X-Forwarded-Proto.
func Origin(r *http.Request) string {
	scheme, host := r.URL.Scheme, r.URL.Host
	if host == "" {
		host = r.Host
	}
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
	}
	return scheme + "://" + host
}
