package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
)

// Every command says how long it took, and each of its parts: a developer and an agent then see where the time
// goes. And it knows what is usual, on this machine, for this app: the last runs of each are kept
// (.charter/times.json), and a run that takes much longer than they did says so, with what is usual beside it. So
// a build that has started to drag is noticed the day it does, not the week after.

// kept is how many runs of a command or a part are remembered.
const kept = 20

// usual is the times of a command's (or a part's) last runs here, by its name ("check", "check/lint").
type usual map[string][]float64

func timesFile() string {
	p, err := project.Load(".")
	if err != nil {
		return ""
	}
	return filepath.Join(p.Root, ".charter", "times.json")
}

func readUsual() usual {
	u := usual{}
	if file := timesFile(); file != "" {
		if b, err := os.ReadFile(file); err == nil {
			json.Unmarshal(b, &u)
		}
	}
	return u
}

// took records that name took d and says whether that is much longer than usual: the usual time, and true, when
// there are at least three runs to compare with, and this one took half as long again and a second more.
func (u usual) took(name string, d time.Duration) (median time.Duration, slow bool) {
	past := u[name]
	if len(past) >= 3 {
		sorted := slices.Clone(past)
		slices.Sort(sorted)
		m := sorted[len(sorted)/2]
		median = time.Duration(m * float64(time.Second))
		slow = d.Seconds() > m*1.5 && d.Seconds()-m > 1
	}
	u[name] = append(past, d.Seconds())
	if len(u[name]) > kept {
		u[name] = u[name][len(u[name])-kept:]
	}
	return median, slow
}

func (u usual) save() {
	file := timesFile()
	if file == "" {
		return
	}
	if b, err := json.Marshal(u); err == nil {
		os.MkdirAll(filepath.Dir(file), 0o755)
		os.WriteFile(file, b, 0o644)
	}
}

// reported is set by a command that says its own time (one with parts), so Finished does not say it again.
var reported bool

// Finished is what the command line says when a command ends: how long it took, and when that is much longer
// than it usually takes here, what is usual. A command with parts has said so itself (times.done).
func Finished(name string, start time.Time) {
	if reported {
		return
	}
	d := time.Since(start)
	u := readUsual()
	median, slow := u.took(name, d)
	u.save()
	switch {
	case slow:
		fmt.Printf("✓ %s took %s: slower than usual here (%s)\n", name, run.Seconds(d), run.Seconds(median))
	case d >= time.Second:
		fmt.Printf("✓ %s took %s\n", name, run.Seconds(d))
	}
}
