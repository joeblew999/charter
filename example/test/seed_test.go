package test

import (
	"strings"
	"testing"

	"github.com/joeblew999/charter/app/apptest"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/pages"
	notes "github.com/joeblew999/charter/example/sdk/go"
	"github.com/joeblew999/charter/example/sdk/go/option"
)

// The Worker under test starts with the seed (api.Seeded): the people and notes a developer's own database starts
// with, which these tests name by the same values. A deployed Worker has no seed, so they run on a local one only.

// The seeded people are there, each with their role, and their notes are on the page under their names.
func TestTheWorkerStartsWithTheSeed(t *testing.T) {
	if !apptest.Local() {
		t.Skip("a deployed Worker has no seed")
	}
	// Ada is the seed's admin: her token says who is what.
	people, err := sdk(t, option.WithToken(api.Seeded.Ada.Token)).People.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]string{}
	for _, p := range people.Data {
		roles[p.ID] = string(p.Role)
	}
	for _, p := range []api.SeededPerson{api.Seeded.Ada, api.Seeded.Grace, api.Seeded.Lin} {
		if roles[p.ID] != p.Role {
			t.Errorf("%s is %q, want %q", p.Name, roles[p.ID], p.Role)
		}
	}
	html := apptest.Get(t, at(pages.Home{}))
	for _, n := range api.Seeded.Notes {
		if !strings.Contains(html, n.By.Name) {
			t.Errorf("the page does not show a note by %s", n.By.Name)
		}
	}
}

// Being one of them is their token: Lin, a member, may not take away Grace's note; Grace, a moderator, may take
// away Lin's. The same rule the page and the API are under everywhere else, with no one to sign up first.
func TestTheSeededPeopleAreWhoTheirRolesSay(t *testing.T) {
	if !apptest.Local() {
		t.Skip("a deployed Worker has no seed")
	}
	lin := sdk(t, option.WithToken(api.Seeded.Lin.Token))
	grace := sdk(t, option.WithToken(api.Seeded.Grace.Token))
	note, err := grace.Notes.Create(t.Context(), &notes.CreateNotesRequest{Body: "Grace's, for this test"})
	if err != nil {
		t.Fatal(err)
	}
	if note.Author != api.Seeded.Grace.Name {
		t.Errorf("the note says %q wrote it", note.Author)
	}
	if err := lin.Notes.Delete(t.Context(), &notes.DeleteNotesRequest{ID: note.ID}); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("Lin removing Grace's note: %v, want 403", err)
	}
	if _, err := lin.People.List(t.Context()); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("Lin listing people: %v, want 403", err)
	}
	mine, err := lin.Notes.Create(t.Context(), &notes.CreateNotesRequest{Body: "Lin's, for this test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := grace.Notes.Delete(t.Context(), &notes.DeleteNotesRequest{ID: mine.ID}); err != nil {
		t.Errorf("Grace removing Lin's note: %v", err)
	}
	if err := grace.Notes.Delete(t.Context(), &notes.DeleteNotesRequest{ID: note.ID}); err != nil {
		t.Errorf("Grace removing her own: %v", err)
	}
}
