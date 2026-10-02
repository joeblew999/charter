//go:build js && wasm

package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"

	"github.com/joeblew999/orpc-api/api/go/internal/promise"
)

// DurableObject is the hub on Cloudflare: the object called name of the Durable Object namespace
// bound as binding (cloudflare.config.ts). The class is worker/hub.mjs, which sends each published
// body to every subscriber as it is and knows nothing of the type, so one class serves every feed:
// each name is its own object, with its own subscribers.
//
// Open it per request: a binding belongs to the request's environment.
func DurableObject[T any](binding, name string) (Hub[T], error) {
	namespace := cloudflare.GetBinding(binding)
	if !namespace.Truthy() {
		return nil, fmt.Errorf("%s is not bound (cloudflare.config.ts)", binding)
	}
	return durableObject[T]{name: name, stub: namespace.Call("get", namespace.Call("idFromName", name))}, nil
}

type durableObject[T any] struct {
	name string
	stub js.Value
}

// Publish is one call of the hub's fetch and one wait, for its status. It does not go through
// net/http: making an http.Request into a JavaScript one, and the answer into an http.Response,
// crosses between Go and JavaScript many times, and each crossing costs CPU.
func (h durableObject[T]) Publish(_ context.Context, item T) error {
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	res, err := promise.Await(h.stub.Call("fetch", "https://hub/publish", map[string]any{"method": "POST", "body": string(body)}))
	if err != nil {
		return fmt.Errorf("hub %s publish: %w", h.name, err)
	}
	if status := res.Get("status").Int(); status != 200 {
		return fmt.Errorf("hub %s publish: HTTP %d", h.name, status)
	}
	return nil
}

// Subscribe opens a WebSocket to the hub (workers-go has no WebSocket client, so this is
// syscall/js). The hub's side hibernates; every published item arrives as one text frame. A close
// or an error (a deploy restarts the hub) is reported once, and Follow resubscribes.
func (h durableObject[T]) Subscribe(listener func(T), onError func(error)) (func(), error) {
	res, err := promise.Await(h.stub.Call("fetch", "https://hub/subscribe", map[string]any{"headers": map[string]any{"Upgrade": "websocket"}}))
	if err != nil {
		return nil, fmt.Errorf("hub %s subscribe: %w", h.name, err)
	}
	socket := res.Get("webSocket")
	if !socket.Truthy() {
		return nil, fmt.Errorf("hub %s subscribe: HTTP %d, no WebSocket", h.name, res.Get("status").Int())
	}
	open := true
	onMessage := js.FuncOf(func(_ js.Value, args []js.Value) any {
		var item T
		if err := json.Unmarshal([]byte(args[0].Get("data").String()), &item); err == nil {
			listener(item)
		}
		return nil
	})
	broken := func(what string) js.Func {
		return js.FuncOf(func(_ js.Value, args []js.Value) any {
			if open {
				open = false
				onError(fmt.Errorf("hub %s socket %s", h.name, what))
			}
			return nil
		})
	}
	onClose, onErr := broken("closed"), broken("failed")
	socket.Call("accept")
	socket.Call("addEventListener", "message", onMessage)
	socket.Call("addEventListener", "close", onClose)
	socket.Call("addEventListener", "error", onErr)
	return func() {
		if open {
			open = false
			func() {
				defer func() { recover() }() // close throws on a socket that is already closing
				socket.Call("close", 1000, "done")
			}()
		}
		onMessage.Release()
		onClose.Release()
		onErr.Release()
	}, nil
}
