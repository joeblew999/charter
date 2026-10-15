package problem

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// A refusal is problem details: its media type, its status in the body as in the answer, and why.
func TestWriteIsProblemDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, 404, "not found")
	var body struct {
		Title  string
		Status int
		Detail string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 404 || rec.Header().Get("Content-Type") != "application/problem+json" || body.Status != 404 || body.Title != "Not Found" || body.Detail != "not found" {
		t.Errorf("HTTP %d %q %+v", rec.Code, rec.Header().Get("Content-Type"), body)
	}
}
