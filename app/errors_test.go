package app

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
)

// One handler answers for every error a page returns: a status, a redirect as the request needs it, a reader who
// left, and anything else as a 500 that shows nothing of the error.
func TestFailed(t *testing.T) {
	ask := func(err error, htmx bool, ctx context.Context) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/x", nil).WithContext(ctx)
		if htmx {
			r.Header.Set("HX-Request", "true")
		}
		w := httptest.NewRecorder()
		failed(w, r, err)
		return w
	}
	bg := context.Background()
	if w := ask(fmt.Errorf("loading: %w", NotFound("no such note")), false, bg); w.Code != 404 {
		t.Errorf("a wrapped NotFound: %d", w.Code)
	}
	if w := ask(Redirect{To: "/de/bindings"}, false, bg); w.Code != 303 || w.Header().Get("Location") != "/de/bindings" {
		t.Errorf("a redirect: %d %q", w.Code, w.Header().Get("Location"))
	}
	if w := ask(Redirect{To: "/bindings"}, true, bg); w.Code != 200 || w.Header().Get("HX-Location") != "/bindings" {
		t.Errorf("a redirect for htmx: %d %q", w.Code, w.Header().Get("HX-Location"))
	}
	if w := ask(errors.New("the database's password is hunter2"), false, bg); w.Code != 500 || w.Body.String() != "Something went wrong.\n" {
		t.Errorf("a plain error: %d %q", w.Code, w.Body.String())
	}
	gone, cancel := context.WithCancel(bg)
	cancel()
	if w := ask(errors.New("x"), false, gone); w.Code != 499 {
		t.Errorf("a reader who left: %d", w.Code)
	}
}
