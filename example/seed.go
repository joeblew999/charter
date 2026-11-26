//go:build seed

package main

import (
	"context"

	charter "github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/api"
	"github.com/joeblew999/charter/example/built"
)

// The seed: what fills the local database (go tool charter run db:seed; charter dev and charter test, on a database
// they have just made). What it fills it with is api.Seeded, written with the app's own types and through its own
// functions; this file only says so to charter. It is built only for seeding (the build tag), in a Worker started
// on this machine: a Worker that is deployed has none of it.
func init() {
	app.Seed("DB", func(ctx context.Context, conn cloud.DB) error {
		// What the app does with each note written, it does for a seeded one: here at once, where a request would
		// leave it to the queue, so the activity page has the seed's notes on it when the seed is done.
		queued := api.OnPost
		api.OnPost = func(ctx context.Context, note api.Note) {
			built.Record(ctx, conn, built.NoteWritten, note.ID, note.Author, 0)
		}
		defer func() { api.OnPost = queued }()
		return api.Seed(ctx, api.StoreOn(conn), charter.AccountsOn(conn))
	})
}
