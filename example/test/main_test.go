package test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/api"
	notes "github.com/joeblew999/charter/example/sdk/go"
	"github.com/joeblew999/charter/example/sdk/go/client"
	"github.com/joeblew999/charter/example/sdk/go/option"
)

// The tests leave the Worker as they found it: whoever one signs up it deletes again (apptest's SignUp), and a note
// it writes it takes away (write). That is what lets the same tests ask a deployed Worker, whose database is real.
// Here it is held to: a local Worker starts with the seed and nothing else, so when the tests are done it has the
// seed's people and notes, and no more.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && os.Getenv("BASE") != "" && apptest.Local() {
		if left := leftBehind(); left != "" {
			fmt.Println("FAIL: the tests left the Worker with more than its seed:" + left)
			code = 1
		}
	}
	os.Exit(code)
}

// leftBehind is what the Worker has that the seed did not put there.
func leftBehind() string {
	ada := client.NewClient(option.WithBaseURL(strings.TrimRight(os.Getenv("BASE"), "/")), option.WithToken(api.Seeded.Ada.Token))
	var left strings.Builder
	people, err := ada.People.List(context.Background())
	if err != nil {
		return " " + err.Error()
	}
	seeded := map[string]bool{api.Seeded.Ada.ID: true, api.Seeded.Grace.ID: true, api.Seeded.Lin.ID: true}
	for _, p := range people.Data {
		if !seeded[p.ID] {
			name := p.ID
			if p.Name != nil {
				name = *p.Name
			}
			fmt.Fprintf(&left, "\n  a person, %s (%s)", name, p.Role)
		}
	}
	page, err := ada.Notes.List(context.Background(), &notes.ListNotesRequest{})
	if err != nil {
		return " " + err.Error()
	}
	written := map[string]bool{}
	for _, n := range api.Seeded.Notes {
		written[n.Body] = true
	}
	for _, n := range page.Results {
		if !written[n.Body] {
			fmt.Fprintf(&left, "\n  a note by %s: %s", n.Author, n.Body)
		}
	}
	return left.String()
}
