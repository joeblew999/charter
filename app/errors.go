package app

import (
	"context"
	"errors"
	"log"
	"net/http"
)

// How a page or a handler of the tree says something went another way than a page, as structpages has it: it returns
// an error, and never writes the response itself. One handler, failed, answers for all of them.

// Status is an error with the HTTP status the answer should have: a page that is not there, a request that is
// refused. A plain error is a 500, logged, with nothing of it shown.
func Status(status int, message string) error { return statusError{status, message} }

// NotFound is Status(404): what a page's Props returns when what was asked for is not there.
func NotFound(what string) error { return statusError{http.StatusNotFound, what} }

type statusError struct {
	status  int
	message string
}

func (e statusError) Error() string { return e.message }

// Redirect is what a handler returns to send the browser somewhere else: not an error, but it takes the error's way
// out, so nothing has been written. An htmx request gets HX-Location (htmx follows it as a navigation; a 3xx would be
// followed by the request itself, and its answer swapped into the wrong place), any other a 303.
type Redirect struct{ To string }

func (Redirect) Error() string { return "redirect" }

// failed answers for a page or handler that returned an error.
func failed(w http.ResponseWriter, r *http.Request, err error) {
	// The reader went away: expected, and no one to answer.
	if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
		w.WriteHeader(499)
		return
	}
	var to Redirect
	if errors.As(err, &to) {
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Location", to.To)
			return
		}
		http.Redirect(w, r, to.To, http.StatusSeeOther)
		return
	}
	var known statusError
	if errors.As(err, &known) {
		http.Error(w, known.message, known.status)
		return
	}
	log.Printf("app: %s %s: %v", r.Method, r.URL.Path, err)
	http.Error(w, "Something went wrong.", http.StatusInternalServerError)
}

// llms is the app's LLMs with the tree's addresses, as seo.Site takes it.
func (a *App) llms() func(origin string) string {
	if a.LLMs == nil {
		return nil
	}
	return func(origin string) string {
		return a.LLMs(origin, func(page any) string {
			if a.tree == nil {
				return ""
			}
			path, err := a.tree.URLFor(page)
			if err != nil {
				log.Printf("app: llms.txt: %v", err)
			}
			return path
		})
	}
}
