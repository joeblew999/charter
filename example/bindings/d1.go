//go:build js && wasm

package bindings

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/syumai/workers-go/cloudflare/d1"
)

// D1 through workers-go's database/sql driver: how many notes the home page's list has.

func rows(ctx context.Context) string {
	connector, err := d1.OpenConnector("DB")
	if err != nil {
		return err.Error()
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notes").Scan(&n); err != nil {
		return err.Error()
	}
	return strconv.Itoa(n)
}
