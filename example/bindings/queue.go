//go:build js && wasm

package bindings

import (
	"strings"

	"github.com/syumai/workers-go/cloudflare/kv"
	"github.com/syumai/workers-go/cloudflare/queues"
)

// Queues: the form is the producer; the consumer puts each message at the top of a short list in KV.

func enqueue(message string) error {
	producer, err := queues.NewProducer("QUEUE")
	if err != nil {
		return err
	}
	return producer.SendText(message)
}

func onQueue() {
	queues.ConsumeNonBlock(func(batch *queues.MessageBatch) error {
		ns, err := namespace()
		if err != nil {
			return err
		}
		lines := queued(ns)
		for _, m := range batch.Messages {
			body, err := m.StringBody()
			if err != nil {
				return err
			}
			lines = append([]string{body}, lines...)
			m.Ack()
		}
		if len(lines) > 5 {
			lines = lines[:5]
		}
		return ns.PutString("queued", strings.Join(lines, "\n"), nil)
	})
}

// queued is what the consumer has taken, newest first.
func queued(ns *kv.Namespace) []string {
	if log := get(ns, "queued"); log != "" {
		return strings.Split(log, "\n")
	}
	return nil
}
