package specfile

import (
	"context"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/orpc-api/go/humaworkers"
)

// The reporter's shape: an id with a pattern, and a section with an enum.
type report struct {
	Serial string `json:"serial" pattern:"^[A-Z]{2}-\\d{4}$"`
	Status string `json:"status" enum:"ok,degraded" example:"fine"`
	Count  int32  `json:"count" minimum:"1" example:"3"`
	Note   string `json:"note"`
}

type reportInput struct {
	ID    string `path:"id" pattern:"^dev-\\d+$"`
	Limit int32  `query:"limit" maximum:"10" example:"50"`
	Body  report
}

type goodInput struct {
	ID   string `path:"id" pattern:"^dev-\\d+$" example:"dev-7"`
	Body struct {
		Serial string `json:"serial" pattern:"^[A-Z]{2}-\\d{4}$" example:"AB-1234"`
		// The list's example is its items' too.
		Modes []string `json:"modes" enum:"ok,degraded" example:"[\"ok\"]"`
		Bad   []string `json:"bad" enum:"ok,degraded" example:"[\"fine\"]"`
	}
}

type reportOutput struct {
	Body struct {
		// Not required of a response field, but checked when given.
		Code string `json:"code" pattern:"^\\d+$" example:"abc"`
		Rank int32  `json:"rank" minimum:"1"`
	}
}

func route[I, O any](id, path string, hidden bool) humaworkers.Route {
	return humaworkers.Route{Method: "POST", Path: path, Register: func(api huma.API) {
		huma.Register(api, huma.Operation{OperationID: id, Method: "POST", Path: path, Hidden: hidden},
			func(context.Context, *I) (*O, error) { return nil, nil })
	}}
}

func TestAConstrainedRequestFieldNeedsAnExampleAndEveryExampleMustBeValid(t *testing.T) {
	var got []string
	for _, problem := range Examples(humaworkers.New(humaworkers.Config("t", "1"), []humaworkers.Route{
		route[reportInput, reportOutput]("report", "/devices/{id}/reports", false),
		route[goodInput, struct{}]("good", "/good/{id}", false),
		// A hidden operation (a WebSocket channel) is checked too.
		route[struct {
			After string `query:"after" pattern:"^\\d+$"`
		}, struct{}]("live", "/live", true),
	})) {
		got = append(got, problem.Error())
	}
	want := []string{
		"report: path.id is constrained and has no example",
		"report: query.limit: example 50 is not valid: expected number <= 10",
		"report: body.serial is constrained and has no example",
		`report: body.status: example "fine" is not valid: expected value to be one of "ok, degraded"`,
		`report: 200 response.code: example "abc" is not valid: expected string to match pattern ^\d+$`,
		`good: body.bad: example ["fine"] is not valid: expected value to be one of "ok, degraded"`,
		"live: query.after is constrained and has no example",
	}
	for _, problem := range got {
		found := false
		for i, prefix := range want {
			if strings.HasPrefix(problem, prefix) {
				want, found = append(want[:i], want[i+1:]...), true
				break
			}
		}
		if !found {
			t.Errorf("not expected: %s", problem)
		}
	}
	for _, missing := range want {
		t.Errorf("not reported: %s", missing)
	}
}
