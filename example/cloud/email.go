//go:build js && wasm

package cloud

import (
	"context"
	"strconv"
	"time"

	"github.com/joeblew999/charter/cloud"
)

// Email, out through the send_email binding MAIL and in from Email Routing. workers-go has no email package yet:
// these two calls are charter's.

func send(to string) error {
	raw := "From: site@example.com\r\nTo: " + to + "\r\nSubject: Hello from charter-site\r\nMessage-ID: <" +
		strconv.FormatInt(time.Now().UnixNano(), 36) + "@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nSent from a Go Worker.\r\n"
	if err := cloud.SendEmail("MAIL", "site@example.com", to, []byte(raw)); err != nil {
		return err
	}
	ns, err := namespace()
	if err != nil {
		return err
	}
	return ns.PutString("sent", "→ "+to, nil)
}

// onEmail keeps who wrote, and the subject.
func onEmail() {
	cloud.ConsumeEmail(func(_ context.Context, m *cloud.EmailMessage) error {
		ns, err := namespace()
		if err != nil {
			return err
		}
		return ns.PutString("received", m.From+": "+m.Header("subject"), nil)
	})
}
