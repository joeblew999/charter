//go:build js && wasm

package cloud

import (
	"github.com/syumai/workers-go/cloudflare/kv"
	"github.com/syumai/workers-go/cloudflare/queues"
	"github.com/syumai/workers-go/cloudflare/r2"
)

// What workers-go opens, opened by the binding's typed name. workers-go's packages are for a Worker only, so these
// are too: call them from a file that is (//go:build js && wasm), as every use of workers-go is.

// Open is workers-go's namespace for the binding.
func (b KV) Open() (*kv.Namespace, error) { return kv.NewNamespace(b.name) }

// Open is workers-go's bucket for the binding.
func (b Bucket) Open() (*r2.Bucket, error) { return r2.NewBucket(b.name) }

// Open is workers-go's producer for the binding.
func (b Queue) Open() (*queues.Producer, error) { return queues.NewProducer(b.name) }
