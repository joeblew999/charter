//go:build js && wasm

package transport

import "runtime"

// maxSys is how much memory a runtime built with the standard compiler may hold from the system and still serve the
// next request. A Worker has 128 MB for everything, the Wasm module and the other waiting runtimes among it.
const maxSys = 48 << 20

// full says whether this runtime should serve no more requests. The standard compiler's collector does its job and its
// heap grows as it needs, so a collection is no reason to go: the runtime goes when it holds too much memory.
func full(stats *runtime.MemStats, _ uint64) bool { return stats.Sys > maxSys }

// fromJS runs f for a call from JavaScript. With the standard compiler such a call must return before JavaScript can
// go on, so nothing in it may wait: a promise it waited for could never settle ("all goroutines are asleep"). f runs
// in a goroutine of its own.
func fromJS(f func()) { go f() }
