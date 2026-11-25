//go:build !(js && wasm)

package intl

// Runtime is goIntl (js/intl-worker.mjs) outside a Worker: intl/workerd serves the same module from workerd, for an
// ordinary Go server (workerd.Start) and for go test (intltest.Main). There is no Intl without it, so using intl panics.
var Runtime func(method string, args ...string) string

func call(method string, args ...string) (string, bool) {
	if Runtime == nil {
		panic("intl: no Intl: outside a Worker, start workerd's first (workerd.Start(); in tests intltest.Main)")
	}
	return Runtime(method, args...), true
}
