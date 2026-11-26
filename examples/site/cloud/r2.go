//go:build js && wasm

package cloud

import (
	"io"
	"strings"

	"github.com/syumai/workers-go/cloudflare/r2"
)

// R2: the note is one object in the bucket.

func note() (string, error) {
	bucket, err := r2.NewBucket("BUCKET")
	if err != nil {
		return "", err
	}
	object, err := bucket.Get("note.txt")
	if err != nil || object == nil {
		return "", nil // no note yet
	}
	b, _ := io.ReadAll(object.Body)
	return string(b), nil
}

func keepNote(text string) error {
	bucket, err := r2.NewBucket("BUCKET")
	if err != nil {
		return err
	}
	_, err = bucket.Put("note.txt", io.NopCloser(strings.NewReader(text)), nil)
	return err
}
