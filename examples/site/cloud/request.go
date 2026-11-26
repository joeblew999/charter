//go:build js && wasm

package cloud

import (
	"context"
	"strings"

	"github.com/syumai/workers-go/cloudflare/fetch"
)

// The request's properties (request.cf), as workers-go reads them.

func properties(ctx context.Context) string {
	p, err := fetch.NewIncomingProperties(ctx)
	if err != nil {
		return err.Error()
	}
	return strings.Join([]string{p.Country, p.City, p.Timezone, p.Colo, p.HttpProtocol}, " · ")
}
