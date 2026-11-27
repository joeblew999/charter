package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

// A request the app cannot answer is logged as one JSON line, with a reference the reader is given too: what was
// asked and what went wrong, and nothing of what was sent.
func TestAFailureIsLoggedWithAReference(t *testing.T) {
	var out bytes.Buffer
	failures = &out
	r := httptest.NewRequest("POST", "/notes?token=secret-in-the-query", strings.NewReader("a body"))
	r.Header.Set("Cf-Ray", "8f3a1b2c3d4e5f60-SIN")
	r.Header.Set("Authorization", "Bearer never-logged")
	w := httptest.NewRecorder()
	failed(w, r, errors.New("the database is away"))

	if w.Code != 500 || !strings.Contains(w.Body.String(), "8f3a1b2c3d4e5f60-SIN") || w.Header().Get("X-Request-Id") != "8f3a1b2c3d4e5f60-SIN" {
		t.Errorf("the reader was told: HTTP %d %q, %q", w.Code, w.Body.String(), w.Header().Get("X-Request-Id"))
	}
	if strings.Contains(w.Body.String(), "database") {
		t.Error("the reader was told what went wrong inside")
	}
	var line map[string]any
	if err := json.Unmarshal(out.Bytes(), &line); err != nil {
		t.Fatalf("the log line is not JSON: %v\n%s", err, out.String())
	}
	if line["level"] != "error" || line["reference"] != "8f3a1b2c3d4e5f60-SIN" || line["path"] != "/notes" || line["error"] != "the database is away" {
		t.Errorf("the log line: %v", line)
	}
	for _, never := range []string{"secret-in-the-query", "never-logged", "a body"} {
		if strings.Contains(out.String(), never) {
			t.Errorf("the log line has %q, which was sent, not asked", never)
		}
	}
	// With no id of Cloudflare's, one is made.
	if ref := Reference(httptest.NewRequest("GET", "/", nil)); len(ref) != 16 {
		t.Errorf("a made reference: %q", ref)
	}
}
