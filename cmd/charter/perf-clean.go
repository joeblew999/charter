package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// charter perf-clean: REMOTE: delete every scratch Worker <worker>-perf-* and its database that a perf run left
// Run by: mise run perf:clean (tasks/repo/tasks.toml)
func init() {
	commands["perf-clean"] = command{"[-all]", "REMOTE: delete every scratch Worker <worker>-perf-* and its database that a perf run left", perfClean}
}

func perfClean(args []string) error {
	var all bool
	flags("perf-clean", args, func(f *flag.FlagSet) {
		f.BoolVar(&all, "all", false, "also delete scratch Workers made in the last 20 minutes: another run may be using one")
	})
	_, worker, err := deployed()
	if err != nil {
		return err
	}
	out, err := output(".", npmBin("cf"), "workers", "list", "--per-page", "100")
	if err != nil {
		return err
	}
	var workers []struct {
		Name      string
		CreatedOn time.Time `json:"created_on"`
	}
	if err := json.Unmarshal([]byte(out), &workers); err != nil {
		return fmt.Errorf("cf workers list: %w", err)
	}
	inUse := map[string]bool{}
	scratch := regexp.MustCompile(`^` + regexp.QuoteMeta(worker) + `-perf-[a-z0-9-]+$`)
	found := 0
	for _, w := range workers {
		if scratch.MatchString(w.Name) {
			if !all && time.Since(w.CreatedOn) < 20*time.Minute {
				fmt.Printf("left %s: made %s ago, a run may be using it (-all deletes it)\n", w.Name, time.Since(w.CreatedOn).Round(time.Second))
				inUse[w.Name+"-db"] = true
				continue
			}
			found++
			if err := deleteScratch(w.Name); err != nil {
				return err
			}
		}
	}
	// A database whose Worker is already gone.
	databases, err := d1Databases()
	if err != nil {
		return err
	}
	for _, d := range databases {
		if name, isDB := strings.CutSuffix(d.Name, "-db"); isDB && scratch.MatchString(name) && !inUse[d.Name] {
			found++
			if err := sh(".", npmBin("cf"), "d1", "delete", d.UUID, "--force"); err != nil {
				return err
			}
		}
	}
	fmt.Printf("%d scratch Workers and databases of %s deleted\n", found, worker)
	return nil
}
