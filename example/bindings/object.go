//go:build js && wasm

package bindings

import (
	"context"
	"net/http"
	"strconv"

	"github.com/syumai/workers-go/cloudflare"
)

// A Durable Object through its stub: the hub that wakes the home page's stream. A plain request is not what it
// serves, so its status is the proof the call arrived.

func object(ctx context.Context) string {
	namespace, err := cloudflare.NewDurableObjectNamespace("HUB")
	if err != nil {
		return err.Error()
	}
	stub, err := namespace.Get(namespace.IdFromName("notes"))
	if err != nil {
		return err.Error()
	}
	ask, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://hub/", nil)
	res, err := stub.Fetch(ask)
	if err != nil {
		return err.Error()
	}
	return "Hub → HTTP " + strconv.Itoa(res.StatusCode)
}
