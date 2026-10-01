//go:build js && wasm

package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"
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
	return durableObject[T]{binding: binding, name: name, namespace: namespace}, nil
}

type durableObject[T any] struct {
	binding, name string
	namespace     js.Value
}

func (h durableObject[T]) Publish(_ context.Context, item T) error {
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	namespace, err := cloudflare.NewDurableObjectNamespace(h.binding)
	if err != nil {
		return err
	}
	stub, err := namespace.Get(namespace.IdFromName(h.name))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://hub/publish", bytes.NewReader(body))
	if err != nil {
		return err
	}
	res, err := stub.Fetch(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("hub %s publish: HTTP %d", h.name, res.StatusCode)
	}
	return nil
}

// Subscribe opens a WebSocket to the hub (workers-go has no WebSocket client, so this is
// syscall/js). The hub's side hibernates; every published item arrives as one text frame. A close
// or an error (a deploy restarts the hub) is reported once, and Follow resubscribes.
func (h durableObject[T]) Subscribe(listener func(T), onError func(error)) (func(), error) {
	object := js.Global().Get("Object")
	headers := object.New()
	headers.Set("Upgrade", "websocket")
	init := object.New()
	init.Set("headers", headers)
	stub := h.namespace.Call("get", h.namespace.Call("idFromName", h.name))
	res, err := await(stub.Call("fetch", "https://hub/subscribe", init))
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

// await waits for a JavaScript promise.
func await(promise js.Value) (js.Value, error) {
	type result struct {
		value js.Value
		err   error
	}
	done := make(chan result, 1)
	then := js.FuncOf(func(_ js.Value, args []js.Value) any {
		value := js.Undefined()
		if len(args) > 0 {
			value = args[0]
		}
		done <- result{value: value}
		return nil
	})
	catch := js.FuncOf(func(_ js.Value, args []js.Value) any {
		message := "rejected"
		if len(args) > 0 {
			message = js.Global().Call("String", args[0]).String()
		}
		done <- result{err: errors.New(message)}
		return nil
	})
	defer then.Release()
	defer catch.Release()
	promise.Call("then", then, catch)
	r := <-done
	return r.value, r.err
}
