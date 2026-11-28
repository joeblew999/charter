//go:build !(js && wasm)

package cloud

import (
	"net/http"
	"time"
)

// Not a Worker: the package compiles, and every call answers ErrNotCloudflare.

// OnWorkers reports whether the code runs as a Worker.
const OnWorkers = false

func Var(string) string                              { return "" }
func allow(string, string) (bool, error)             { return true, ErrNotCloudflare }
func sendEmail(string, string, string, []byte) error { return ErrNotCloudflare }
func HTTPClient() *http.Client                       { return &http.Client{} }
func StaticAssets(string) http.Handler               { return http.NotFoundHandler() }
func open(string) (DB, error)                        { return DB{}, ErrNotCloudflare }
func Query[T any](DB, string, ...any) ([]T, error)   { return nil, ErrNotCloudflare }
func Exec(DB, string, ...any) error                  { return ErrNotCloudflare }
func ExecRows(DB, string, ...any) (int64, error)     { return 0, ErrNotCloudflare }
func Batch(DB, ...Statement) error                   { return ErrNotCloudflare }

// A request's session, which only a Worker has (session.go).
func Begin(Bookmarks)    {}
func End()               {}
func Newest()            {}
func Reached() Bookmarks { return nil }
func Brought() Bookmarks { return nil }
func Served() []string   { return nil }

// DB is a D1 database.
type DB struct{}

func Bindings() []Binding                 { return nil }
func Probe(string) (time.Duration, error) { return 0, ErrNotCloudflare }
func Runtimes() (int, int64, int64, bool) { return 0, 0, 0, false }
