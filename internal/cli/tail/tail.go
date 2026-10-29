// Package tail is go tool charter tail: it streams a deployed Worker's live logs (requests, console output, exceptions) without
// wrangler: flags for internal/cftail. Ctrl-C stops it and deletes the tail.
//
// Credentials come from CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID, e.g. with `fnox exec --`.
package tail

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/joeblew999/charter/internal/cftail"
	"github.com/joeblew999/charter/project"
)

// Main runs the command with args, the arguments after its name.
func Main(args []string) {
	fs := flag.NewFlagSet("charter tail", flag.ExitOnError)
	log.SetFlags(0)
	name := fs.String("name", os.Getenv("APP_NAME"), "Worker script name (default $APP_NAME, else charter.toml's name)")
	asJSON := fs.Bool("json", false, "print raw events as JSON lines")
	duration := fs.Duration("for", 0, "stop after this long (0 = until Ctrl-C)")
	fs.Parse(args)
	if *name == "" {
		if p, err := project.Load("."); err == nil {
			*name = p.Name
		}
	}
	if *name == "" {
		log.Fatal("tail: -name is required (or a charter.toml)")
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
