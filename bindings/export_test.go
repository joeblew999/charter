//go:build !(js && wasm)

package bindings

// NotUnderTest makes Var answer as it does outside go test, until the returned function is called.
func NotUnderTest() (restore func()) {
	was := underTest
	underTest = func() bool { return false }
	return func() { underTest = was }
}
