// Package pages is the site: HTML pages rendered on the server from gsx components (the *.gsx files;
// go tool gsx generate writes the *.x.go beside them), made live with htmx 4. It serves:
//
//	GET  /                  the home page: the form and the newest messages (Home)
//	POST /messages          the form: stores the message as POST /api/messages does, answers the form again (Form)
//	GET  /messages/stream   every new message as an Item fragment, over SSE (stream.go)
//	GET  /cloud             what Cloudflare gives the Worker, one card each (cloud.gsx, package cloud)
//
// and hands everything else to the API. The pages call the API's handlers in the process (api.Env),
// not over HTTP.
package pages

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/examples/site/api"
)

// The newest messages the home page shows.
const shown = 50

// Handler serves the pages, and next (the API) for every other path.
func Handler(env api.Env, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method := r.URL.Path, r.Method
		switch {
		case path == "/" && method == http.MethodGet:
			home(w, r, env)
		case path == "/messages" && method == http.MethodPost:
			post(w, r, env)
		case path == "/messages/stream" && method == http.MethodGet:
			serveStream(w, r, env, htmxEvent)
		case path == "/cloud" || strings.HasPrefix(path, "/cloud/"):
			serveCloud(w, r)
		case path == "/" || strings.HasPrefix(path, "/messages"):
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func home(w http.ResponseWriter, r *http.Request, env api.Env) {
	messages, err := env.Recent(r.Context(), shown)
	if err != nil {
		fail(w, err)
		return
	}
	// The stream starts after the newest message shown: nothing between the two is lost.
	var after int64
	if len(messages) > 0 {
		after = messages[0].ID
	}
	render(w, r, http.StatusOK, Home(messages, after))
}

func post(w http.ResponseWriter, r *http.Request, env api.Env) {
	body := r.PostFormValue("body")
	_, err := env.Post(r.Context(), body)
	switch {
	case errors.Is(err, api.ErrBody):
		// htmx 4 swaps a 422 too: the form comes back with what was typed and why it was refused.
		render(w, r, http.StatusUnprocessableEntity, Form(body, err.Error()))
	case err != nil:
		fail(w, err)
	default:
		// An empty form. The message itself reaches this page as it reaches every other: by the stream.
		render(w, r, http.StatusOK, Form("", ""))
	}
}

// render writes a component as the answer: whole, or a 500 if it fails half-way.
func render(w http.ResponseWriter, r *http.Request, status int, node gsx.Node) {
	var html bytes.Buffer
	if err := node.Render(r.Context(), &html); err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// With its length a page goes out whole, not in chunks (htmx's history restore wants that on Workers).
	w.Header().Set("Content-Length", strconv.Itoa(html.Len()))
	w.WriteHeader(status)
	w.Write(html.Bytes())
}

func fail(w http.ResponseWriter, err error) {
	log.Printf("pages: %v", err)
	http.Error(w, "Something went wrong.", http.StatusInternalServerError)
}
