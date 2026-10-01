//go:build js && wasm

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"syscall/js"

	"github.com/syumai/workers-go/cloudflare"
	"github.com/syumai/workers-go/cloudflare/d1"

	"github.com/joeblew999/orpc-api/api-go/api"
)

// env binds the API to the Worker's bindings (cloudflare.config.ts): APP_NAME, DB (D1), HUB (NotesHub).
func env() api.Env {
	return api.Env{
		Var: cloudflare.Getenv,
		Store: func() (api.Store, error) {
			connector, err := d1.OpenConnector("DB")
			if err != nil {
				return nil, fmt.Errorf("DB is not bound (cloudflare.config.ts): %w", err)
			}
			return api.SQLStore{DB: sql.OpenDB(connector)}, nil
		},
		Hub: func() (api.Hub, error) {
			namespace := cloudflare.GetBinding("HUB")
			if !namespace.Truthy() {
				return nil, errors.New("HUB is not bound (cloudflare.config.ts)")
			}
			return hub{namespace}, nil
		},
	}
}

// hub is the NotesHub Durable Object (worker/hub.mjs): one object, named "notes".
type hub struct{ namespace js.Value }

func (h hub) stub() js.Value {
	return h.namespace.Call("get", h.namespace.Call("idFromName", "notes"))
}

func (h hub) Publish(_ context.Context, note api.Note) error {
	body, err := json.Marshal(note)
	if err != nil {
		return err
	}
	namespace, err := cloudflare.NewDurableObjectNamespace("HUB")
	if err != nil {
		return err
	}
	stub, err := namespace.Get(namespace.IdFromName("notes"))
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
		return fmt.Errorf("hub publish: HTTP %d", res.StatusCode)
	}
	return nil
}

// Subscribe opens a WebSocket to the hub (workers-go has no WebSocket client, so this is
// syscall/js). The hub's side hibernates; every published note arrives as one text frame. A close
// or an error (a deploy restarts the hub) is reported once, and Follow resubscribes.
func (h hub) Subscribe(listener func(api.Note), onError func(error)) (func(), error) {
	object := js.Global().Get("Object")
	headers := object.New()
	headers.Set("Upgrade", "websocket")
	init := object.New()
	init.Set("headers", headers)
	res, err := await(h.stub().Call("fetch", "https://hub/subscribe", init))
	if err != nil {
		return nil, fmt.Errorf("hub subscribe: %w", err)
	}
	socket := res.Get("webSocket")
	if !socket.Truthy() {
		return nil, fmt.Errorf("hub subscribe: HTTP %d, no WebSocket", res.Get("status").Int())
	}
	open := true
	onMessage := js.FuncOf(func(_ js.Value, args []js.Value) any {
		var note api.Note
		if err := json.Unmarshal([]byte(args[0].Get("data").String()), &note); err == nil {
			listener(note)
		}
		return nil
	})
	broken := func(what string) js.Func {
		return js.FuncOf(func(_ js.Value, args []js.Value) any {
			if open {
				open = false
				onError(fmt.Errorf("hub socket %s", what))
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
