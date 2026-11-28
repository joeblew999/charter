//go:build !(js && wasm)

package live

import "github.com/joeblew999/charter/cloud"

// DurableObject is for a Worker: a program built for this machine has no Durable Object (Memory is the hub of a
// test).
func DurableObject[T any](cloud.Object, string) (Hub[T], error) { return nil, cloud.ErrNotCloudflare }
