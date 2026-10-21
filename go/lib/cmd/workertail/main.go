// Command workertail streams a deployed Worker's live logs (requests, console output, exceptions) without
// wrangler: flags for internal/cftail. Ctrl-C stops it and deletes the tail.
//
// Credentials come from CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID, e.g. with `fnox exec --`.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/joeblew999/charter/go/lib/internal/cftail"
)

func main() {
	log.SetFlags(0)
	name := flag.String("name", os.Getenv("APP_NAME"), "Worker script name (default $APP_NAME)")
	asJSON := flag.Bool("json", false, "print raw events as JSON lines")
	duration := flag.Duration("for", 0, "stop after this long (0 = until Ctrl-C)")
	flag.Parse()
	if *name == "" {
		log.Fatal("tail: -name is required")
	}
	c := &cftail.Client{Token: os.Getenv("CLOUDFLARE_API_TOKEN"), AccountID: os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
		Ready: func() { log.Printf("tailing %s (Ctrl-C to stop)", *name) }}
	if c.Token == "" || c.AccountID == "" {
		log.Fatal("tail: CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID must be set (use fnox exec)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}
	err := c.Tail(ctx, *name, func(e cftail.Event) {
		if *asJSON {
			b, _ := json.Marshal(e)
			fmt.Println(string(b))
			return
		}
		fmt.Println(cftail.Format(e))
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tail stopped at %s", time.Now().Format(time.TimeOnly))
}
