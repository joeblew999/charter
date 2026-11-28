//go:build js && wasm

package built

import (
	"context"
	"strings"
	"time"

	"github.com/syumai/workers-go/cloudflare/kv"

	"github.com/joeblew999/charter/example/env"
)

// KV: a line for everyone, shown on the notes page. It is read on every visit and written rarely, by whoever may
// moderate, and it does not matter that a change takes up to a minute to show everywhere. That is what a key-value
// store is for, and what it is not for: KV keeps answering with a value for a while after it has changed, so nothing
// that must be read back at once belongs in it (the notes, and who may do what, are in the database).

func store() (*kv.Namespace, error) { return env.KV.Open() }

// text is what KV has under key, "" when it has nothing: workers-go gives a missing key as the text "<null>".
func text(ns *kv.Namespace, key string) string {
	v, err := ns.GetString(key, nil)
	if err != nil || v == "<null>" {
		return ""
	}
	return v
}

// NoticeNow is the notice, empty when there is none.
func NoticeNow(context.Context) (Notice, error) {
	ns, err := store()
	if err != nil {
		return Notice{}, err
	}
	// When it was set, then the line.
	at, line, _ := strings.Cut(text(ns, "notice"), "\n")
	set, _ := time.Parse(time.RFC3339, at)
	return Notice{Text: line, Set: set}, nil
}

// SetNotice shows a line to everyone; an empty one takes the notice away.
func SetNotice(_ context.Context, line string) (Notice, error) {
	ns, err := store()
	if err != nil {
		return Notice{}, err
	}
	line = strings.TrimSpace(strings.ReplaceAll(line, "\n", " "))
	if r := []rune(line); len(r) > MaxNotice {
		line = string(r[:MaxNotice])
	}
	if line == "" {
		return Notice{}, ns.Delete("notice")
	}
	now := time.Now().UTC().Truncate(time.Second)
	return Notice{Text: line, Set: now}, ns.PutString("notice", now.Format(time.RFC3339)+"\n"+line, nil)
}
