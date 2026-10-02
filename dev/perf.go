package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// perf is the quick loop for performance work on Cloudflare, made so that several people or agents
// can each try a change at the same time: every run has a Worker of its own, named after the
// experiment, with its own database and hub, and removes them when it is done.
//
// It needs the Worker's cloudflare.config.ts to name the Worker after the mode when the mode
// starts with "perf-" (api/go's does), and API_GO_URL: the scratch Worker's URL is that URL with
// the scratch name as its first label.

func init() {
	commands["perf"] = command{"-name <experiment> [-dir api/go] [-build '<wasm-build flags>'] [-prebuilt] [-keep] [-- <bench flags>]",
		"REMOTE: build, deploy to a scratch Worker <worker>-perf-<experiment>, bench it from its first request, delete it (-keep: leave it)", perf}
	commands["perf-clean"] = command{"[-worker <name>]", "REMOTE: delete every scratch Worker <worker>-perf-* and its database that a perf run left", perfClean}
}

var experimentName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,20}[a-z0-9])?$`)

func perf(args []string) error {
	name, dir, build, keep, prebuilt := "", "api/go", "", false, false
	rest := flags("perf", args, func(f *flag.FlagSet) {
		f.StringVar(&name, "name", "", "the experiment: lower-case letters, digits and hyphens, at most 22. Its Worker is <worker>-perf-<name>")
		f.StringVar(&dir, "dir", dir, "the folder of the Go Worker")
		f.StringVar(&build, "build", "", "flags for wasm-build, e.g. '-plain' or '-heap 4 -stack 96kb'")
		f.BoolVar(&prebuilt, "prebuilt", false, "deploy the build/ that is there (made by another TinyGo, say) instead of building")
		f.BoolVar(&keep, "keep", false, "leave the scratch Worker and its database (perf-clean removes them later)")
	})
	if !experimentName.MatchString(name) {
		return errors.New("perf needs -name <experiment>: lower-case letters, digits and hyphens, e.g. -name stack96")
	}
	deployed, err := url.Parse(os.Getenv("API_GO_URL"))
	if err != nil || deployed.Host == "" {
		return errors.New("perf needs API_GO_URL, the deployed Worker's URL: the scratch Worker's is derived from it")
	}
	base, domain, _ := strings.Cut(deployed.Host, ".")
	mode := "perf-" + name
	worker := base + "-" + mode
	scratch := "https://" + worker + "." + domain

	if !prebuilt {
		if err := wasmBuild(append([]string{"-dir", dir}, strings.Fields(build)...)); err != nil {
			return err
		}
	}
	if !keep {
		defer func() {
			if err := deleteScratch(dir, worker); err != nil {
				fmt.Println("not cleaned up (dev perf-clean does it):", err)
			}
		}()
	}
	if err := quiet(dir, nil, cf, "deploy", "--mode", mode); err != nil {
		return err
	}
	if err := migrate([]string{"-worker", worker}); err != nil {
		return err
	}
	// A new workers.dev name takes a while to answer.
	fmt.Printf("deployed %s; waiting for it to answer...\n", scratch)
	for deadline := time.Now().Add(90 * time.Second); ; time.Sleep(2 * time.Second) {
		res, err := benchClient.Get(scratch + "/api/openapi.json")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not answer in 90 s", scratch)
		}
	}
	// Deployed again, so that the bench meets a new isolate: the requests above used the first.
	if err := quiet(dir, nil, cf, "deploy", "--mode", mode); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	if len(rest) == 0 {
		rest = []string{"-each", "-burst", "8", "-write"}
	}
	return bench(append(rest, "-worker", worker, scratch))
}

func perfClean(args []string) error {
	var worker string
	flags("perf-clean", args, func(f *flag.FlagSet) {
		f.StringVar(&worker, "worker", "", "the Worker whose scratch Workers to delete (default: the first label of API_GO_URL's host)")
	})
	if worker == "" {
		if deployed, err := url.Parse(os.Getenv("API_GO_URL")); err == nil {
			worker, _, _ = strings.Cut(deployed.Host, ".")
		}
	}
	if worker == "" {
		return errors.New("perf-clean needs -worker <name> or API_GO_URL")
	}
	dir := "api/go"
	out, err := output(dir, cf, "workers", "list", "--per-page", "100")
	if err != nil {
		return err
	}
	var workers []struct{ Name string }
	if err := json.Unmarshal([]byte(out), &workers); err != nil {
		return fmt.Errorf("cf workers list: %w", err)
	}
	scratch := regexp.MustCompile(`^` + regexp.QuoteMeta(worker) + `-perf-[a-z0-9-]+$`)
	found := 0
	for _, w := range workers {
		if scratch.MatchString(w.Name) {
			found++
			if err := deleteScratch(dir, w.Name); err != nil {
				return err
			}
		}
	}
	// A database whose Worker is already gone.
	databases, err := d1Databases(dir)
	if err != nil {
		return err
	}
	for _, d := range databases {
		if name, isDB := strings.CutSuffix(d.Name, "-db"); isDB && scratch.MatchString(name) {
			found++
			if err := sh(dir, cf, "d1", "delete", d.UUID, "--force"); err != nil {
				return err
			}
		}
	}
	fmt.Printf("%d scratch Workers and databases of %s deleted\n", found, worker)
	return nil
}

// deleteScratch deletes a scratch Worker and its D1 database. It refuses any other name.
func deleteScratch(dir, worker string) error {
	if !strings.Contains(worker, "-perf-") {
		return fmt.Errorf("%s is not a scratch Worker", worker)
	}
	if err := quiet(dir, nil, cf, "workers", "delete", worker, "--force"); err != nil {
		return err
	}
	databases, err := d1Databases(dir)
	if err != nil {
		return err
	}
	for _, d := range databases {
		if d.Name == worker+"-db" {
			if err := quiet(dir, nil, cf, "d1", "delete", d.UUID, "--force"); err != nil {
				return err
			}
		}
	}
	fmt.Println("deleted", worker, "and its database")
	return nil
}

type d1Database struct{ Name, UUID string }

func d1Databases(dir string) ([]d1Database, error) {
	out, err := output(dir, cf, "d1", "list", "--per-page", "100")
	if err != nil {
		return nil, err
	}
	var databases []d1Database
	if err := json.Unmarshal([]byte(out), &databases); err != nil {
		return nil, fmt.Errorf("cf d1 list: %w", err)
	}
	return databases, nil
}
