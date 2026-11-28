// Package accounts is the tables charter's sign-in keeps an app's people in, and every statement charter runs on
// them, typed: what an app has for its own tables (db/schema.sql, db/queries.sql, db/queries.gen.go), charter has
// here for its own. No statement on these tables is written in Go.
//
// schema.sql is what charter add screen:sign-in writes into an app's db/schema.sql. queries.gen.go is written
// from queries.sql against it by the generator that writes an app's (internal/dbcode, with sqlc): a test fails
// while it is not what they give, and UPDATE=1 go test ./internal/accounts writes it. The app owns its copy of the
// tables, so charter check runs these queries against the app's schema too (dbcode.Fits).
package accounts

import _ "embed"

// Schema is the tables, as SQL.
//
//go:embed schema.sql
var Schema string

// Queries is the statements, as an app's db/queries.sql has its own.
//
//go:embed queries.sql
var Queries string
