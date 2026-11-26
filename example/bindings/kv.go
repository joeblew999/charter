//go:build js && wasm

package bindings

import (
	"log"
	"strconv"

	"github.com/syumai/workers-go/cloudflare/kv"
)

// KV: a counter, and where the other cards keep what happened outside a request.

func namespace() (*kv.Namespace, error) { return kv.NewNamespace("KV") }

// visit counts this visit and gives the count. Cloudflare takes one write a second to a key (429 beyond it): a visit
// within the same second as another is shown and not counted.
func visit(ns *kv.Namespace) int {
	n, _ := strconv.Atoi(get(ns, "visits"))
	n++
	if err := ns.PutString("visits", strconv.Itoa(n), nil); err != nil {
		log.Printf("bindings: visit not counted: %v", err)
	}
	return n
}

// get is what KV has under key, "" when it has nothing: workers-go gives a missing key as the text "<null>".
func get(ns *kv.Namespace, key string) string {
	v, err := ns.GetString(key, nil)
	if err != nil || v == "<null>" {
		return ""
	}
	return v
}
