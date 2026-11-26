//go:build js && wasm

package built

import (
	"context"
	"strings"
	"time"

	"github.com/syumai/workers-go/cloudflare/kv"
)

// KV: the note someone has typed and not posted. It is kept under their id for a week, so it is there when they
// come back, on any device; posting the note takes it away. KV is made for this: read far more often than written,
// and nothing that must be exact.

func drafts() (*kv.Namespace, error) { return kv.NewNamespace("KV") }

// text is what KV has under key, "" when it has nothing: workers-go gives a missing key as the text "<null>".
func text(ns *kv.Namespace, key string) string {
	v, err := ns.GetString(key, nil)
	if err != nil || v == "<null>" {
		return ""
	}
	return v
}

// DraftOf is someone's draft, empty when they have none.
func DraftOf(_ context.Context, user string) (Draft, error) {
	if user == "" {
		return Draft{}, nil
	}
	ns, err := drafts()
	if err != nil {
		return Draft{}, err
	}
	// When it was saved, then the text.
	at, body, _ := strings.Cut(text(ns, "draft:"+user), "\n")
	saved, _ := time.Parse(time.RFC3339, at)
	return Draft{Text: body, Saved: saved}, nil
}

// SaveDraft keeps what someone has typed; nothing typed takes the draft away.
func SaveDraft(_ context.Context, user, body string) (Draft, error) {
	ns, err := drafts()
	if err != nil || user == "" {
		return Draft{}, err
	}
	if strings.TrimSpace(body) == "" {
		return Draft{}, ns.Delete("draft:" + user)
	}
	now := time.Now().UTC().Truncate(time.Second)
	err = ns.PutString("draft:"+user, now.Format(time.RFC3339)+"\n"+body, &kv.PutOptions{ExpirationTTL: 7 * 24 * 3600})
	return Draft{Text: body, Saved: now}, err
}
