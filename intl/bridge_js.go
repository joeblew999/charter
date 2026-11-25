//go:build js && wasm

package intl

import "syscall/js"

// call calls goIntl[method] (js/intl-worker.mjs, on globalThis) with string arguments; ok is
// false when there is none or it throws.
func call(method string, args ...string) (s string, ok bool) {
	g := js.Global().Get("goIntl")
	if g.IsUndefined() || g.IsNull() {
		return "", false
	}
	defer func() {
		if recover() != nil {
			s, ok = "", false
		}
	}()
	in := make([]any, len(args))
	for i, a := range args {
		in[i] = a
	}
	return g.Call(method, in...).String(), true
}
