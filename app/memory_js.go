//go:build js && wasm

package app

import "runtime"

// maxSys is how much memory a runtime built with the standard compiler may hold from the system and still serve the
// next request. A Worker has 128 MB for everything, the Wasm module and the other waiting runtimes among it.
const maxSys = 48 << 20

// full says whether this runtime should serve no more requests. The standard compiler's collector does its job and its
// heap grows as it needs, so a collection is no reason to go: the runtime goes when it holds too much memory.
func full(stats *runtime.MemStats, _ uint64) bool { return stats.Sys > maxSys }
