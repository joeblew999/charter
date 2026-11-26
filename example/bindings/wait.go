//go:build js && wasm

package bindings

import (
	"time"

	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/kv"
)

// waitUntil: work that goes on after this answer has left. The page shows when the last such work finished.

func afterAnswer(ns *kv.Namespace) string {
	last := get(ns, "after")
	cloudflare.WaitUntil(func() {
		ns.PutString("after", time.Now().UTC().Format(time.RFC3339Nano), nil)
	})
	return last
}
