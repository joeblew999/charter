// Package logs is charter logs: a deployed Worker's stored logs (Workers Logs, which every charter deploy turns on),
// for what happened before now: after a failed live test or deploy, "what did it log in the last ten minutes, errors
// only". charter tail is the live stream. Flags for internal/cflogs.
//
// Credentials come from CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID, e.g. with `fnox exec --`.
package logs

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joeblew999/charter/internal/cflogs"
	"github.com/joeblew999/charter/project"
)

// Main runs the command with args, the arguments after its name.
func Main(args []string) {
	fs := flag.NewFlagSet("charter logs", flag.ExitOnError)
	log.SetFlags(0)
	q := cflogs.Query{}
	fs.StringVar(&q.Script, "name", os.Getenv("APP_NAME"), "Worker script name (default $APP_NAME, else charter.toml's name)")
	fs.DurationVar(&q.Since, "since", 10*time.Minute, "how far back to read")
	fs.StringVar(&q.Level, "level", "", "only this level: error, warn, info, log, debug")
	fs.StringVar(&q.Search, "search", "", "only events with this text")
	fs.IntVar(&q.Status, "status", 0, "only requests answered with this status, e.g. 500")
	fs.IntVar(&q.Limit, "limit", 50, fmt.Sprintf("at most this many events, the newest (at most %d)", cflogs.MaxLimit))
	asJSON := fs.Bool("json", false, "print events as JSON lines")
	fs.Parse(args)
	if q.Script == "" {
		if p, err := project.Load("."); err == nil {
			q.Script = p.Name
		}
	}
	if q.Script == "" {
		log.Fatal("logs: -name is required (or a charter.toml)")
	}
	c := &cflogs.Client{Token: os.Getenv("CLOUDFLARE_API_TOKEN"), AccountID: os.Getenv("CLOUDFLARE_ACCOUNT_ID")}
	if c.Token == "" || c.AccountID == "" {
		log.Fatal("logs: CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID must be set (use fnox exec)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	events, err := c.Events(ctx, q)
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range events {
		if *asJSON {
			b, _ := json.Marshal(e)
			fmt.Println(string(b))
			continue
		}
		fmt.Println(cflogs.Format(e))
	}
	// Say what was asked and how much came back: none is an answer too, and a full limit means there is more.
	about := fmt.Sprintf("%d events of %s in the last %s", len(events), q.Script, q.Since)
	if len(events) == q.Limit {
		about += " (the limit: there may be more; -limit, or a shorter -since)"
	}
	log.Print(about)
}
