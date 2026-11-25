// Package deps is charter outdated and charter update: a repo's pins against what upstream has, and moving them all
// in one step, except what deps.toml holds and why.
//
//	charter outdated            every pin with a newer version upstream, the held ones marked
//	charter update [-no-check]  move each that isn't held, go mod tidy in every module, then charter check
//
// The pins: the Go modules each of the repo's modules requires directly or for a tool (every go.mod git tracks), and
// mise.toml's tools. Charter's own two pins, its library in go.mod and its command in mise.toml, move together, and
// charter init then writes charter's files again. What a repo pins elsewhere (a vendored file, a lockfile of its own)
// is the repo's to move: its own update command, which charter.toml's [ci] update names for the weekly job.
//
// deps.toml, at the repo's top, is what stays where it is:
//
//	[hold]
//	"github.com/chromedp/chromedp" = "v0.17 rewrote its API: #170"   # a Go module, or a tool as mise.toml names it
//	"@moq/*" = "…"                                                    # a name ending in * holds every name that starts so
//	[copies]
//	workerd = ["intl/workerd/workerd.go"]                             # files that write a pin's version too, quoted
//	"*" = ["internal/pins/repo.toml"]                                 # files written as mise.toml is: every tool they pin
//	skip = ["examples/notes-go/sdk/go"]                               # folders whose go.mod is generated: not the repo's to move
package deps

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joeblew999/charter/internal/cli/repo"
	"github.com/joeblew999/charter/internal/cli/self"
	"github.com/joeblew999/charter/internal/deps"
)

// Outdated runs charter outdated with args. It exits 1 when a pin that isn't held is behind, so a script can tell.
func Outdated(args []string) {
	fs := flag.NewFlagSet("charter outdated", flag.ExitOnError)
	fs.Parse(args)
	log.SetFlags(0)
	changes, _, err := pass(false)
	if err != nil {
		log.Fatal("✗ ", err)
	}
	deps.Report("Behind upstream (charter update moves every one not held):", changes)
	for _, c := range changes {
		if c.Held == "" {
			os.Exit(1)
		}
	}
}

// Update runs charter update with args.
func Update(args []string) {
	fs := flag.NewFlagSet("charter update", flag.ExitOnError)
	noCheck := fs.Bool("no-check", false, "move the pins and tidy, without running charter check after")
	fs.Parse(args)
	log.SetFlags(0)
	changes, root, err := pass(true)
	deps.Report("Moved (and held):", changes)
	if err != nil {
		log.Fatal("✗ ", err)
	}
	moved := 0
	for _, c := range changes {
		if c.Held == "" {
			moved++
		}
	}
	if moved == 0 {
		fmt.Println("\nNothing to move: every pin is at its latest, or held.")
		return
	}
	if *noCheck {
		fmt.Printf("\n%d moved. Not checked (-no-check): charter check before committing.\n", moved)
		return
	}
	fmt.Printf("\n%d moved. Checking: charter check\n", moved)
	if err := os.Chdir(root); err != nil {
		log.Fatal(err)
	}
	if _, err := self.Output("check"); err != nil {
		log.Printf("✗ charter check fails with the moved pins: fix it, or hold what broke it in deps.toml ([hold], with why)")
		os.Exit(self.ExitCode(err))
	}
	fmt.Println("✓ every pin moved passes charter check: commit them")
}

// pass looks at every pin of the repo the working directory is in, moving them when update, and returns what it
// found (or moved) and the repo's root.
func pass(update bool) ([]deps.Change, string, error) {
	root, err := repo.Root()
	if err != nil {
		return nil, "", err
	}
	conf, err := deps.Read(root)
	if err != nil {
		return nil, root, err
	}
	mods, err := deps.Modules(root, conf.Skip...)
	if err != nil {
		return nil, root, err
	}
	o := deps.Options{Update: update, Warn: func(err error) { log.Printf("· %v", err) }}
	changes, err := deps.GoModules(root, mods, conf, o)
	if err != nil {
		return changes, root, err
	}
	tools, err := deps.MiseTools(root, mods, conf, o)
	changes = append(changes, tools...)
	if err != nil {
		return changes, root, err
	}
	if update {
		err = deps.Tidy(mods)
	}
	return changes, root, err
}
