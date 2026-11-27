//go:build !(js && wasm)

package cloud

import (
	"database/sql"
	"net/http"
	"time"
)

// Not a Worker: the package compiles, and every call answers ErrNotCloudflare.

// OnWorkers reports whether the code runs as a Worker.
const OnWorkers = false

func Var(string) string                            { return "" }
func Allow(string, string) (bool, error)           { return true, ErrNotCloudflare }
func HTTPClient() *http.Client                     { return &http.Client{} }
func StaticAssets(string) http.Handler             { return http.NotFoundHandler() }
func Open(string) (DB, error)                      { return DB{}, ErrNotCloudflare }
func SQL(string) (*sql.DB, error)                  { return nil, ErrNotCloudflare }
func Query[T any](DB, string, ...any) ([]T, error) { return nil, ErrNotCloudflare }
func Exec(DB, string, ...any) error                { return ErrNotCloudflare }
func ExecRows(DB, string, ...any) (int64, error)   { return 0, ErrNotCloudflare }
func Batch(DB, ...Statement) error                 { return ErrNotCloudflare }

// DB is a D1 database.
type DB struct{}

func Bindings() []Binding                 { return nil }
func Probe(string) (time.Duration, error) { return 0, ErrNotCloudflare }
func Runtimes() (int, int64, int64, bool) { return 0, 0, 0, false }
