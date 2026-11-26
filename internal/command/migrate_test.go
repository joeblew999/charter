package command

import "testing"

// An app written for an older charter: its imports and the names that moved with them, and nothing else.
func TestMoveImports(t *testing.T) {
	before := `package app

import (
	"github.com/joeblew999/charter/site/intl"
	"github.com/joeblew999/charter/follow"
	"github.com/joeblew999/charter/hub"
	"github.com/joeblew999/charter/web"
	"github.com/joeblew999/charter/problem"
)

func serve(w http.ResponseWriter, r *http.Request, hub hub.Hub[Item]) {
	hub.Publish(r.Context(), Item{})
	follow.Follow(r.Context(), nil, follow.Options{}, nil)
	problem.Write(w, 404, web.Origin(r))
	_ = intl.Text
}
`
	after := `package app

import (
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/live"
	"github.com/joeblew999/charter/transport"
	"github.com/joeblew999/charter/humaworkers"
)

func serve(w http.ResponseWriter, r *http.Request, hub live.Hub[Item]) {
	hub.Publish(r.Context(), Item{})
	live.Follow(r.Context(), nil, live.Options{}, nil)
	humaworkers.WriteProblem(w, 404, transport.Origin(r))
	_ = intl.Text
}
`
	if got := moveImports(before); got != after {
		t.Fatalf("got:\n%s", got)
	}
}

// An older charter.toml: the contract's keys go under [check] with their comment, [sdks] is [pack].
func TestMoveSettings(t *testing.T) {
	before := `# What charter adds.
health = "/healthz"
url = "https://app.example.com"
# The API's contract.
spec = "fern/openapi.json"
db = "sqlc"

[sdks]
name = "Notes"
`
	after := `# What charter adds.
health = "/healthz"
url = "https://app.example.com"

[check]
# The API's contract.
spec = "fern/openapi.json"
db = "sqlc"

[pack]
name = "Notes"
`
	if got := moveSettings(before); got != after {
		t.Fatalf("got:\n%s", got)
	}
	if again := moveSettings(after); again != after {
		t.Fatalf("a second time:\n%s", again)
	}
}

// The app has a package called live of its own: charter's is imported under another name.
func TestMoveImportsBesideTheAppsOwn(t *testing.T) {
	before := `import (
	"github.com/joeblew999/charter/follow"
	"github.com/joeblew999/charter/hub"
	"example.com/app/internal/live"
)

var _ = hub.Memory[live.Message]{}
var _ = follow.Options{}
`
	after := `import (
	charterlive "github.com/joeblew999/charter/live"
	"example.com/app/internal/live"
)

var _ = charterlive.Memory[live.Message]{}
var _ = charterlive.Options{}
`
	if got := moveImports(before); got != after {
		t.Fatalf("got:\n%s", got)
	}
}
