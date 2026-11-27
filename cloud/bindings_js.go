//go:build js && wasm

package cloud

import (
	"errors"
	"slices"
	"syscall/js"
	"time"

	"github.com/joeblew999/charter/internal/promise"
)

// Bindings are what the Worker is bound to, as it is running: each name of its environment, and what kind of thing
// is there. It is read from the environment itself, not from wrangler.jsonc, so it is what the Worker really has.
// A variable and a secret are both text there: Set says whether one has a value, which is never given.
func Bindings() []Binding {
	env := js.Global().Get("context").Get("env")
	keys := js.Global().Get("Object").Call("keys", env)
	var out []Binding
	for i := range keys.Length() {
		name := keys.Index(i).String()
		v := env.Get(name)
		b := Binding{Name: name, Kind: kindOf(v)}
		if b.Kind == KindText {
			b.Set = v.String() != ""
		}
		out = append(out, b)
	}
	slices.SortFunc(out, func(a, b Binding) int {
		if a.Kind != b.Kind {
			return slices.Index(kinds, a.Kind) - slices.Index(kinds, b.Kind)
		}
		if a.Name < b.Name {
			return -1
		}
		return 1
	})
	return out
}

// kindOf is what a value of the environment is: by the class Cloudflare's runtime made it of, and where that says
// nothing, by what it can do. A stub that answers to any name at all (a binding wrangler dev stands in for over
// RPC) can do everything and so says nothing either: it is told by its class alone, or is KindOther.
func kindOf(v js.Value) string {
	if v.Type() == js.TypeString {
		return KindText
	}
	if v.Type() != js.TypeObject && v.Type() != js.TypeFunction {
		return KindOther
	}
	if kind, ok := classes[className(v)]; ok {
		return kind
	}
	can := func(method string) bool { return v.Get(method).Type() == js.TypeFunction }
	if can("charterNoSuchMethod") {
		return KindOther
	}
	switch {
	case can("prepare") && can("batch"):
		return KindDatabase
	case can("getWithMetadata"):
		return KindKV
	case can("createMultipartUpload") || can("head") && can("put"):
		return KindBucket
	case can("sendBatch"):
		return KindQueue
	case can("idFromName"):
		return KindDurableObject
	case can("limit"):
		return KindRateLimit
	case can("send"):
		return KindEmail
	case can("fetch"):
		return KindService
	}
	return KindOther
}

// classes are the runtime's classes of bindings, by name.
var classes = map[string]string{
	"D1Database": KindDatabase, "KvNamespace": KindKV, "R2Bucket": KindBucket, "WorkerQueue": KindQueue,
	"DurableObjectNamespace": KindDurableObject, "SendEmail": KindEmail, "Fetcher": KindService, "RateLimit": KindRateLimit,
}

// className is the name of the class a value was made of, "" when it has none to say.
func className(v js.Value) string {
	c := v.Get("constructor")
	if c.Type() != js.TypeFunction {
		return ""
	}
	if name := c.Get("name"); name.Type() == js.TypeString {
		return name.String()
	}
	return ""
}

// ClassOf is the runtime's class of the binding called name: what Bindings went by, for a binding it could not
// tell the kind of.
func ClassOf(name string) string { return className(js.Global().Get("context").Get("env").Get(name)) }

// Probe asks the binding called name one small thing, and says how long the answer took: a database SELECT 1, a
// KV namespace and a bucket a key that is not there. ErrNoProbe: a binding of a kind that has nothing harmless to
// ask (a queue takes only messages).
func Probe(name string) (time.Duration, error) {
	v := js.Global().Get("context").Get("env").Get(name)
	if v.IsUndefined() {
		return 0, errors.New(name + " is not bound")
	}
	var ask js.Value
	switch kindOf(v) {
	case KindDatabase:
		ask = v.Call("prepare", "SELECT 1").Call("first")
	case KindKV:
		ask = v.Call("get", "charter-probe")
	case KindBucket:
		ask = v.Call("head", "charter-probe")
	default:
		return 0, ErrNoProbe
	}
	start := time.Now()
	_, err := promise.Await(ask)
	return time.Since(start), err
}
