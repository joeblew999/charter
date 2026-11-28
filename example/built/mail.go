//go:build js && wasm

package built

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/env"
)

// Email, out through the send_email binding MAIL and in from Email Routing (workers-go has no email package yet:
// the two calls are charter's). What was last sent and last received is kept in KV, each with its time.

// MailNow is the last message sent and the last received.
func MailNow(context.Context) (Mail, error) {
	ns, err := store()
	if err != nil {
		return Mail{}, err
	}
	var m Mail
	at, to, _ := strings.Cut(text(ns, "mail:sent"), "\n")
	m.Sent, _ = time.Parse(time.RFC3339, at)
	m.SentTo = to
	at, rest, _ := strings.Cut(text(ns, "mail:received"), "\n")
	m.Received, _ = time.Parse(time.RFC3339, at)
	m.From, m.Subject, _ = strings.Cut(rest, "\n")
	if !m.Sent.IsZero() && env.MAIL_UNLIMITED.Value() == "" {
		m.MayNotSendBefore = m.Sent.Add(time.Hour)
	}
	return m, nil
}

// SendMail sends one message from MAIL_FROM (a variable: an address on a domain with Email Routing) to MAIL_TO (a
// secret: the owner's address, verified in Email Routing, the only kind Cloudflare delivers to). A public page that
// sends email is a way to fill an inbox, so it sends at most one an hour; asked sooner, it does nothing.
func SendMail(ctx context.Context) error {
	now, err := MailNow(ctx)
	if err != nil || time.Now().Before(now.MayNotSendBefore) {
		return err
	}
	ns, err := store()
	if err != nil {
		return err
	}
	from, to := env.MAIL_FROM.Value(), env.MAIL_TO.Value()
	raw := "From: " + from + "\r\nTo: " + to + "\r\nSubject: Hello from " + env.APP_NAME.Value() + "\r\nMessage-ID: <" +
		strconv.FormatInt(time.Now().UnixNano(), 36) + "@" + from[strings.LastIndex(from, "@")+1:] + ">\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nSent from a Go Worker.\r\n"
	if err := env.MAIL.Send(from, to, []byte(raw)); err != nil {
		return err
	}
	return ns.PutString("mail:sent", time.Now().UTC().Format(time.RFC3339)+"\n"+short(to), nil)
}

// onEmail keeps who wrote, and the subject, of each message Email Routing hands the Worker.
func onEmail() {
	cloud.ConsumeEmail(func(_ context.Context, m *cloud.EmailMessage) error {
		ns, err := store()
		if err != nil {
			return err
		}
		return ns.PutString("mail:received", time.Now().UTC().Format(time.RFC3339)+"\n"+short(m.From)+"\n"+m.Header("subject"), nil)
	})
}

// short is an address with its first letter and its domain: the page is public.
func short(address string) string {
	name, domain, _ := strings.Cut(address, "@")
	if name != "" {
		name = name[:1] + "…"
	}
	return name + "@" + domain
}
