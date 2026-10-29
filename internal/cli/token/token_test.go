package token

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer good" {
			w.Write([]byte(`{"success":true,"result":{"status":"active"}}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false,"result":{}}`))
	}))
	defer srv.Close()
	api = srv.URL
	if err := Check("good", "acct"); err != nil {
		t.Errorf("an active token: %v", err)
	}
	if err := Check("bad", "acct"); err == nil || !strings.Contains(err.Error(), "not active") {
		t.Errorf("a bad token: %v", err)
	}
	if err := Check("", "acct"); err == nil {
		t.Error("no token passed")
	}
}
