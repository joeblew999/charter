//go:build js && wasm

// Package email is Cloudflare's email for a Go Worker, which workers-go does not have yet: sending through a
// send_email binding, and receiving what Email Routing hands the Worker. It is written the way workers-go writes its
// own bindings (a name from wrangler.jsonc, a consumer set before the program serves), and like them it builds only
// for a Worker.
//
//	// wrangler.jsonc: "send_email": [{ "name": "MAIL" }]
//	err := email.Send("MAIL", "app@example.com", "you@example.com", raw)   // raw: an RFC 5322 message
//
//	email.ConsumeNonBlock(func(ctx context.Context, m *email.Message) error {
//		raw, err := m.Raw()
//		…
//		return m.Forward("inbox@example.com")
//	})
//
// https://developers.cloudflare.com/email-routing/email-workers/
//
// Upstream: syumai/workers-go has no email package (when it has one: use it, and this goes)
package email

import (
	"context"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/joeblew999/charter/internal/promise"
)

var (
	runtime = js.Global().Get("context")
	binding = runtime.Get("binding")
)

// Send sends raw, a whole message as RFC 5322 has it (headers, a blank line, the body), from one address to another
// through the send_email binding called name. Cloudflare delivers only to addresses the binding allows.
func Send(name, from, to string, raw []byte) error {
	sender := runtime.Get("env").Get(name)
	if sender.IsUndefined() {
		return fmt.Errorf("email: no send_email binding called %s in wrangler.jsonc", name)
	}
	class := binding.Get("EmailMessage")
	if class.IsUndefined() {
		return errors.New("email: the Worker's entry has no EmailMessage: charter build writes one that has")
	}
	_, err := promise.Await(sender.Call("send", class.New(from, to, string(raw))))
	return err
}

// Message is an email Email Routing handed the Worker.
type Message struct {
	From string // the envelope's sender
	To   string // the envelope's recipient
	v    js.Value
}

// Header is the message's header called name, "" without one.
func (m *Message) Header(name string) string {
	if v := m.v.Get("headers").Call("get", name); v.Type() == js.TypeString {
		return v.String()
	}
	return ""
}

// Raw is the whole message as it arrived.
func (m *Message) Raw() ([]byte, error) {
	buf, err := promise.Await(js.Global().Get("Response").New(m.v.Get("raw")).Call("arrayBuffer"))
	if err != nil {
		return nil, err
	}
	bytes := js.Global().Get("Uint8Array").New(buf)
	out := make([]byte, bytes.Get("length").Int())
	js.CopyBytesToGo(out, bytes)
	return out, nil
}

// Reject refuses the message: the sender's server is told reason.
func (m *Message) Reject(reason string) { m.v.Call("setReject", reason) }

// Forward sends the message on to an address verified in Email Routing.
func (m *Message) Forward(to string) error {
	_, err := promise.Await(m.v.Call("forward", to))
	return err
}

// Consumer is what the Worker does with a message it receives.
type Consumer func(ctx context.Context, m *Message) error

// ConsumeNonBlock sets the Worker's consumer of received email. Call it before the program serves (transport.Run, or
// workers-go's workers.Serve), as workers-go's queues.ConsumeNonBlock is.
func ConsumeNonBlock(f Consumer) {
	binding.Set("handleEmail", js.FuncOf(func(_ js.Value, args []js.Value) any {
		message := args[0]
		return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, settle []js.Value) any {
			resolve, reject := settle[0], settle[1]
			go func() {
				m := &Message{From: message.Get("from").String(), To: message.Get("to").String(), v: message}
				if err := f(context.Background(), m); err != nil {
					reject.Invoke(js.Global().Get("Error").New(err.Error()))
					return
				}
				resolve.Invoke(js.Undefined())
			}()
			return js.Undefined()
		}))
	}))
}
