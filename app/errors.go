package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"

	"github.com/jackielii/structpages"

	"github.com/joeblew999/charter/intl"
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

// Auth is who may do what: api.Auth, which is what checks the API's operations too. Allow is nil when the request's
// caller has the scopes; else an error with the status 401 (nobody is signed in) or 403 (someone who may not).
type Auth interface {
	Allow(r *http.Request, needs ...string) error
}

// allowed checks what a route needs of its caller before it runs: the scopes its type's Needs method lists, and
// those of every group above it in the tree. A route with no Needs, under none, is everyone's.
func (a *App) allowed(next http.Handler, pn *structpages.PageNode) http.Handler {
	var needs []string
	found := false
	for node := pn; node != nil; node = node.Parent {
		if n, ok := node.Value.Interface().(interface{ Needs() []string }); ok {
			found = true
			needs = append(needs, n.Needs()...)
		}
	}
	if !found {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.Auth == nil {
			a.failed(w, r, Status(http.StatusInternalServerError, "a page says what it needs of its caller, and the app has no Auth to ask"))
			return
		}
		if err := a.Auth.Allow(r, needs...); err != nil {
			a.failed(w, r, err)
			return
		}
		// What a signed-in reader sees is theirs: no cache keeps it for another.
		w.Header().Set("Cache-Control", "private, no-store")
		next.ServeHTTP(w, r)
	})
}

// failed answers for a page or handler that returned an error. Someone who must sign in first is sent to the
// app's sign-in page, and comes back to where they were going.
func (a *App) failed(w http.ResponseWriter, r *http.Request, err error) {
	var known statusError
	if errors.As(err, &known) && known.status == http.StatusUnauthorized && a.SignIn != nil && a.tree != nil {
		if to, urlErr := structpages.URLFor(r.Context(), a.SignIn); urlErr == nil {
			back := intl.URL(r.Context(), r.URL.RequestURI())
			if r.Method != http.MethodGet {
				back = r.Header.Get("HX-Current-URL") // a form: back to the page it is on
			}
			if back != "" {
				to += "?return=" + url.QueryEscape(intl.SafeReturn(back))
			}
			err = Redirect{To: to}
		}
	}
	failed(w, r, err)
}

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

// llms is /llms.txt as seo.Site takes it: the app's own LLMs, with the tree's addresses; else the docs'.
func (a *App) llms() func(origin string) string {
	if a.LLMs == nil {
		if a.Docs != nil {
			return a.Docs.llms
		}
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
