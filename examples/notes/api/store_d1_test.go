package api

import (
	"context"
	"testing"

	"github.com/joeblew999/charter/cf/d1"
)

// TestD1Store: the queries the Worker runs (api/queries.sql, as charter spec wrote them) on the project's own
// migrations. Off Workers d1.Open("DB") is SQLite in memory, migrated as charter.toml's [[d1]] says, so the SQL that
// runs on Cloudflare is the SQL tested here.
func TestD1Store(t *testing.T) {
	db, err := d1.Open("DB")
	if err != nil {
		t.Fatal(err)
	}
	store, ctx := D1Store{DB: db}, context.Background()
	if latest, err := store.Latest(ctx); latest != 0 || err != nil {
		t.Fatalf("no notes yet: latest %d, %v", latest, err)
	}
	var ids []int64
	for _, body := range []string{"first", "second", "third"} {
		note, err := store.Create(ctx, body)
		if err != nil || note.Body != body || note.CreatedAt == "" {
			t.Fatalf("create %q: %+v, %v", body, note, err)
		}
		ids = append(ids, note.ID)
	}
	if latest, err := store.Latest(ctx); latest != ids[2] || err != nil {
		t.Errorf("latest %d, %v; want %d", latest, err, ids[2])
	}
	before, err := store.Before(ctx, ids[2], 10)
	if err != nil || len(before) != 2 || before[0].Body != "second" || before[1].Body != "first" {
		t.Errorf("before the third, newest first: %+v, %v", before, err)
	}
	since, err := store.Since(ctx, ids[0], 1)
	if err != nil || len(since) != 1 || since[0].Body != "second" {
		t.Errorf("after the first, one: %+v, %v", since, err)
	}
}
