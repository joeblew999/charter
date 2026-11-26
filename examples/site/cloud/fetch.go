//go:build js && wasm

package cloud

import (
	"io"
	"strconv"

	"github.com/syumai/workers-go/cloudflare/fetch"
)

// fetch: a request the Worker makes, to another site (Cloudflare refuses one to the Worker's own address).

// elsewhere is IANA's example domain.
const elsewhere = "https://example.com/"

func fetched() string {
	res, err := fetch.NewClient().HTTPClient(fetch.RedirectModeFollow).Get(elsewhere)
	if err != nil {
		return err.Error()
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	return "GET " + elsewhere + " → " + strconv.Itoa(res.StatusCode)
}
