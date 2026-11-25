package pagespeed_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joeblew999/charter/internal/seokit/pagespeed"
)

// TestAuth: an API key goes as ?key=, a token as Authorization: Bearer, and neither when there is none.
func TestAuth(t *testing.T) {
	var key, header string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, header = r.URL.Query().Get("key"), r.Header.Get("Authorization")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	defer func(e string) { pagespeed.Endpoint = e }(pagespeed.Endpoint)
	pagespeed.Endpoint = srv.URL
	for _, tc := range []struct {
		auth        pagespeed.Auth
		key, header string
	}{
		{pagespeed.Auth{Key: "k123"}, "k123", ""},
		{pagespeed.Auth{Token: "t456"}, "", "Bearer t456"},
		{pagespeed.Auth{}, "", ""},
	} {
		pagespeed.Run(context.Background(), srv.Client(), tc.auth, "https://example.com/", "mobile")
		if key != tc.key || header != tc.header {
			t.Errorf("%+v: key %q, Authorization %q; want %q, %q", tc.auth, key, header, tc.key, tc.header)
		}
	}
}
