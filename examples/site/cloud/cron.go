//go:build js && wasm

package cloud

import (
	"context"
	"time"

	"github.com/syumai/workers-go/cloudflare/cron"
)

// A Cron Trigger (wrangler.jsonc's triggers): which schedule ran, and when.

func onCron() {
	cron.ScheduleTaskNonBlock(func(ctx context.Context) error {
		event, err := cron.NewEvent(ctx)
		if err != nil {
			return err
		}
		ns, err := namespace()
		if err != nil {
			return err
		}
		return ns.PutString("cron", event.Cron+"  "+time.Now().UTC().Format(time.RFC3339), nil)
	})
}
