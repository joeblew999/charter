//go:build js && wasm

package cloud

import (
	"context"
	"strconv"
	"strings"
	"time"

	charter "github.com/joeblew999/charter/cloud"
	"github.com/syumai/workers-go/cloudflare"
)

// Email, out through the send_email binding MAIL and in from Email Routing. workers-go has no email package yet:
// these two calls are charter's.

// send sends one message from MAIL_FROM (a variable: an address on a domain with Email Routing) to the address
// given, or without one to MAIL_TO (a secret: the owner's address, verified in Email Routing, which is the only kind
// Cloudflare delivers to). A public page that sends email is a way to fill an inbox, so it sends at most one an
// hour; asked sooner, it leaves the last one standing.
func send(to string) error {
	ns, err := namespace()
	if err != nil {
		return err
	}
	if last, err := time.Parse(time.RFC3339, strings.SplitN(get(ns, "sent"), " ", 2)[0]); err == nil && time.Since(last) < time.Hour && cloudflare.Getenv("MAIL_UNLIMITED") == "" {
		return nil
	}
	from := cloudflare.Getenv("MAIL_FROM")
	if to == "" {
		to = cloudflare.Getenv("MAIL_TO")
	}
	raw := "From: " + from + "\r\nTo: " + to + "\r\nSubject: Hello from charter-example\r\nMessage-ID: <" +
		strconv.FormatInt(time.Now().UnixNano(), 36) + "@" + from[strings.LastIndex(from, "@")+1:] + ">\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nSent from a Go Worker.\r\n"
	if err := charter.SendEmail("MAIL", from, to, []byte(raw)); err != nil {
		return err
	}
	// When, and to whom, with the address shortened: the page is public.
	name, domain, _ := strings.Cut(to, "@")
	return ns.PutString("sent", time.Now().UTC().Format(time.RFC3339)+" → "+name[:1]+"…@"+domain, nil)
}

// onEmail keeps who wrote, and the subject.
func onEmail() {
	charter.ConsumeEmail(func(_ context.Context, m *charter.EmailMessage) error {
		ns, err := namespace()
		if err != nil {
			return err
		}
		return ns.PutString("received", m.From+": "+m.Header("subject"), nil)
	})
}
