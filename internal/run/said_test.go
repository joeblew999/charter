package run

import (
	"bytes"
	"sync"
	"testing"
)

// What a server prints is passed on as it is, and the first line with one of the texts is kept whole, however the
// output is cut into writes.
func TestSaid(t *testing.T) {
	var out bytes.Buffer
	var mu sync.Mutex
	var rest []byte
	found := ""
	w := said{&out, []string{"fatal error: "}, &mu, &rest, &found}
	for _, part := range []string{"[wrangler] GET / 200\nfatal er", "ror: stopTheWorld: broken CPU ti", "me accounting\n\ngoroutine 8:\nfatal error: another\n"} {
		w.Write([]byte(part))
	}
	if found != "fatal error: stopTheWorld: broken CPU time accounting" {
		t.Errorf("found %q", found)
	}
	if out.String() != "[wrangler] GET / 200\nfatal error: stopTheWorld: broken CPU time accounting\n\ngoroutine 8:\nfatal error: another\n" {
		t.Errorf("passed on %q", out.String())
	}
}
