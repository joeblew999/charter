package web_test

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/joeblew999/charter/go/lib/web"
)

type page string

func (p page) Render(_ context.Context, w io.Writer) error {
	if p == "" {
		return errors.New("no page")
	}
	_, err := io.WriteString(w, string(p))
	return err
}

func TestRender(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := web.Render(rec, httptest.NewRequest("GET", "/", nil), page("<p>hi</p>")); err != nil || rec.Body.String() != "<p>hi</p>" ||
		rec.Header().Get("Content-Length") != "9" || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("render: %v %q %v", err, rec.Body.String(), rec.Header())
	}
	rec = httptest.NewRecorder()
	if err := web.Render(rec, httptest.NewRequest("GET", "/", nil), page("")); err == nil || rec.Code != 500 {
		t.Errorf("a failed render: %v, %d", err, rec.Code)
	}
}

func TestAllowAndOrigin(t *testing.T) {
	rec := httptest.NewRecorder()
	if web.Allow(rec, httptest.NewRequest("HEAD", "/", nil), "GET") != true {
		t.Error("HEAD of a GET route refused")
	}
	rec = httptest.NewRecorder()
	if web.Allow(rec, httptest.NewRequest("DELETE", "/", nil), "GET", "POST") || rec.Code != 405 || rec.Header().Get("Allow") != "GET, POST" {
		t.Errorf("DELETE: %d %q", rec.Code, rec.Header().Get("Allow"))
	}
	r := httptest.NewRequest("GET", "/x", nil)
	r.Host = "app.local:8787"
	if web.Origin(r) != "http://app.local:8787" {
		t.Errorf("Origin = %s", web.Origin(r))
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if web.Origin(r) != "https://app.local:8787" {
		t.Errorf("Origin behind TLS = %s", web.Origin(r))
	}
}
