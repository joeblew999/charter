package command

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"
)

// Go on Workers lives within three limits, and an app drifts towards each a little at a time: the size of what is
// deployed (10 MB gzipped on the paid plan, 3 on the free one), the CPU time the Worker takes to start (a second),
// and the memory a Go runtime holds (of 128 MB for all of them). Each is a number charter already has in passing:
// the build knows the size, wrangler says the start-up time as it deploys, and the Worker says what its runtimes
// hold. So each is said where it is known, kept with the last ones (.charter/times.json), and remarked on when it
// has grown or is near its limit. That costs no time, and is what says, the day it happens, that a limit is coming.

const (
	paidSize   = 10e6 // bytes, gzipped
	startLimit = 1000 // ms of CPU
	nearLimit  = 0.8  // of a limit: worth saying
)

// How much above what is usual here a number must be to be worth saying, for each: as much as it varies by itself
// and more. The size of the same code is the same every time; the CPU a start takes varied from 142 to 224 ms over
// deploys of the same version; what a Go runtime holds grows with the requests it has served.
const (
	sizeGrew    = 1.10
	startGrew   = 1.50
	runtimeGrew = 1.30
)

// measured records a number under a name and gives what is usual for it here: the median of the last ones, known
// once there are three.
func measured(name string, value float64) (usually float64, known bool) {
	u := readUsual()
	past := u["measure/"+name]
	if len(past) >= 3 {
		sorted := slices.Clone(past)
		slices.Sort(sorted)
		usually, known = sorted[len(sorted)/2], true
	}
	u["measure/"+name] = append(past, value)
	if len(u["measure/"+name]) > kept {
		u["measure/"+name] = u["measure/"+name][len(u["measure/"+name])-kept:]
	}
	u.save()
	return usually, known
}

// remark is what to say of a number beside it: that it has grown by more than grew from what is usual here, and that
// it is near its limit. Empty when it is neither.
func remark(value, usually float64, known bool, grew, limit float64, unit func(float64) string) string {
	var said string
	if known && value > usually*grew {
		said += fmt.Sprintf("; up from %s, the usual here", unit(usually))
	}
	if limit > 0 && value > limit*nearLimit {
		said += fmt.Sprintf("; %.0f%% of the %s a Worker may have", 100*value/limit, unit(limit))
	}
	return said
}

func megabytes(bytes float64) string { return fmt.Sprintf("%.2f MB", bytes/1e6) }
func millis(ms float64) string       { return fmt.Sprintf("%.0f ms", ms) }

// startUp is the CPU time the Worker took to start, as wrangler deploy says it.
var startUp = regexp.MustCompile(`Worker Startup Time:\s*(\d+)\s*ms`)

// startedIn says the Worker's start-up time from what wrangler deploy printed, with what is usual and its limit.
func startedIn(deploy string) {
	m := startUp.FindStringSubmatch(deploy)
	if m == nil {
		return
	}
	ms, _ := strconv.ParseFloat(m[1], 64)
	usually, known := measured("start-up", ms)
	say("· the Worker starts in %s of CPU, of the %s Cloudflare allows%s", millis(ms), millis(startLimit), remark(ms, usually, known, startGrew, startLimit, millis))
}

// runtimeHeld is what the Worker says of its Go runtimes in an answer's x-go-runtime (worker/go.mjs).
var runtimeHeld = regexp.MustCompile(`(\d+) runtimes?, (\d+) MB of (\d+)`)

// runtimesHold asks the Worker what its Go runtimes hold, and says what one holds and how many fit: the number
// that decides how much the Worker can do at once. Nothing is said of a Worker that does not tell.
func runtimesHold(base, path, where string) {
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		return
	}
	req.Header.Set("x-go-runtime", "?")
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return
	}
	res.Body.Close()
	m := runtimeHeld.FindStringSubmatch(res.Header.Get("x-go-runtime"))
	if m == nil {
		return
	}
	count, _ := strconv.ParseFloat(m[1], 64)
	held, _ := strconv.ParseFloat(m[2], 64)
	budget, _ := strconv.ParseFloat(m[3], 64)
	if count == 0 || held == 0 {
		return
	}
	each := held / count
	usually, known := measured("runtime-memory/"+where, each)
	grown := ""
	if known && each > usually*runtimeGrew {
		grown = fmt.Sprintf("; up from %.0f MB, the usual here", usually)
	}
	say("· a Go runtime holds about %.0f MB: %d fit in the %.0f MB the Worker gives them, so it answers that many at once in one instance%s", each, int(budget/each), budget, grown)
}
