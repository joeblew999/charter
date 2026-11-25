// Package tail is charter tail: it streams a deployed Worker's live logs (requests, console output, exceptions) without
// wrangler: flags for internal/cftail, its filters among them (-status error, -method POST, -search text, …), which
// Cloudflare applies, so a busy Worker's tail keeps what is asked for instead of a sample. Ctrl-C stops it and deletes the tail.
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
	"strings"
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
	status := fs.String("status", "", "only these outcomes: ok, error, canceled (several with commas)")
	method := fs.String("method", "", "only these HTTP methods (several with commas)")
	header := fs.String("header", "", `only requests with this header: "Name", or "Name: text" in its value`)
	search := fs.String("search", "", "only events with this text in a console message")
	ip := fs.String("ip", "", `only requests from these addresses (several with commas); "self" is yours`)
	sampling := fs.Float64("sampling", 0, "send this share of the events, above 0 and below 1 (0 = all)")
	version := fs.String("version", "", "only this version id of the Worker")
	fs.Parse(args)
	list := func(s string) []string {
		var out []string
		for _, part := range strings.Split(s, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out
	}
	filter := cftail.Filter{Status: list(*status), Methods: list(strings.ToUpper(*method)), Header: *header, Search: *search, IPs: list(*ip), Sampling: *sampling, Version: *version}
	if err := filter.Check(); err != nil {
		log.Fatalf("tail: -%v", err)
	}
	if *name == "" {
		if p, err := project.Load("."); err == nil {
			*name = p.Name
		}
	}
	if *name == "" {
		log.Fatal("tail: -name is required (or a charter.toml)")
	}
	c := &cftail.Client{Token: os.Getenv("CLOUDFLARE_API_TOKEN"), AccountID: os.Getenv("CLOUDFLARE_ACCOUNT_ID"), Filter: filter,
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
