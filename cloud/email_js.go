//go:build js && wasm

// Email: Cloudflare's email for a Go Worker, which workers-go does not have yet: sending through a
// send_email binding, and receiving what Email Routing hands the Worker. It is written the way workers-go writes its
// own bindings (a name from wrangler.jsonc, a consumer set before the program serves), and like them it builds only
// for a Worker.
//
//	// wrangler.jsonc: "send_email": [{ "name": "MAIL" }]
//	err := env.MAIL.Send("app@example.com", "you@example.com", raw)   // raw: an RFC 5322 message
//
//	ConsumeEmail(func(ctx context.Context, m *EmailMessage) error {
//		raw, err := m.Raw()
//		…
//		return m.Forward("inbox@example.com")
//	})
//
// https://developers.cloudflare.com/email-routing/email-workers/
//
// Upstream: syumai/workers-go has no email package (when it has one: use it, and this goes)

package cloud

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

// sendEmail sends raw through the send_email binding called name (Mailer.Send).
func sendEmail(name, from, to string, raw []byte) error {
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

// EmailMessage is an email Email Routing handed the Worker.
type EmailMessage struct {
	From string // the envelope's sender
	To   string // the envelope's recipient
	v    js.Value
}

// Header is the message's header called name, "" without one.
func (m *EmailMessage) Header(name string) string {
	if v := m.v.Get("headers").Call("get", name); v.Type() == js.TypeString {
		return v.String()
	}
	return ""
}

// Raw is the whole message as it arrived.
func (m *EmailMessage) Raw() ([]byte, error) {
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
func (m *EmailMessage) Reject(reason string) { m.v.Call("setReject", reason) }

// Forward sends the message on to an address verified in Email Routing.
func (m *EmailMessage) Forward(to string) error {
	_, err := promise.Await(m.v.Call("forward", to))
	return err
}

// EmailConsumer is what the Worker does with a message it receives.
type EmailConsumer func(ctx context.Context, m *EmailMessage) error

// ConsumeEmail sets the Worker's consumer of received email. Call it before the program serves (app.Run, or
// workers-go's workers.Serve), as workers-go's queues.ConsumeEmail is.
func ConsumeEmail(f EmailConsumer) {
	binding.Set("handleEmail", js.FuncOf(func(_ js.Value, args []js.Value) any {
		message := args[0]
		return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, settle []js.Value) any {
			resolve, reject := settle[0], settle[1]
			go func() {
				m := &EmailMessage{From: message.Get("from").String(), To: message.Get("to").String(), v: message}
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
