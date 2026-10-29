//go:build js && wasm

// Package promise waits for JavaScript promises: the one place the Go Worker's own packages do it
// (hub and d1).
package promise

import (
	"errors"
	"syscall/js"
)

// Await waits for a JavaScript promise: its value, or its rejection as an error.
//
// Under TinyGo every call from JavaScript into Go starts a goroutine, and a settled promise is
// such a call. So await once per piece of work: one statement, one publish.
func Await(promise js.Value) (js.Value, error) {
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
