package humaworkers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

type formInput struct {
	Body struct {
		Name string   `json:"name" minLength:"2"`
		Tags []string `json:"tags,omitempty"`
	} `contentType:"application/x-www-form-urlencoded"`
}

type formOutput struct {
	Body struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
}

func formAPI() *API {
	return New(WithForm(Config("t", "1")), []Route{{Method: http.MethodPost, Path: "/form", Register: func(api huma.API) {
		huma.Register(api, huma.Operation{OperationID: "form", Method: http.MethodPost, Path: "/form"},
			func(_ context.Context, in *formInput) (*formOutput, error) {
				out := &formOutput{}
				out.Body.Name, out.Body.Tags = in.Body.Name, in.Body.Tags
				return out, nil
			})
	}}})
}

func post(api *API, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	return rec
}

func TestAFormBodyIsDecodedAndValidatedLikeAJSONOne(t *testing.T) {
	api := formAPI()
	rec := post(api, FormContentType, "name=a+b%26c&tags=x&tags=y")
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"name":"a b&c","tags":["x","y"]}` {
		t.Fatalf("HTTP %d %s", rec.Code, rec.Body)
	}
	// The response is JSON, also for a client that sends no Accept header.
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("response Content-Type %q", got)
	}
	for body, location := range map[string]string{"name=a": "body.name", "tags=x": "body"} {
		rec := post(api, FormContentType+"; charset=utf-8", body)
		if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"location":"`+location+`"`) {
			t.Errorf("%s: HTTP %d %s, want 422 at %s", body, rec.Code, rec.Body, location)
		}
	}
	if rec := post(api, FormContentType, "name=%zz"); rec.Code != 400 {
		t.Errorf("a malformed form: HTTP %d, want 400", rec.Code)
	}
}

func TestTheSpecNamesTheFormMediaType(t *testing.T) {
	raw, err := json.Marshal(formAPI().OpenAPI())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"requestBody":{"content":{"application/x-www-form-urlencoded":{"schema":`) {
		t.Errorf("the request body is not form-encoded in the spec: %s", raw)
	}
}

func TestWithFormLeavesTheConfigItWasGivenAlone(t *testing.T) {
	config := Config("t", "1")
	before := len(config.Formats)
	if with := WithForm(config); len(config.Formats) != before || len(with.Formats) != before+1 {
		t.Errorf("formats: %d before, %d after, %d in the new config", before, len(config.Formats), len(with.Formats))
	}
}
