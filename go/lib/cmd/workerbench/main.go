// Command workerbench measures the CPU a local Worker spends per request: the operating system's
// count of workerd's CPU time before and after n requests of one kind, divided by n. CPU, not
// waiting, is what Cloudflare bills a Worker for and what its limits count; measured locally, without
// Cloudflare, it is the same for anyone who runs it, so a comparison can be checked:
//
//	npm run dev -w go/examples-notes &                    # or any Worker under workerd
//	go run github.com/joeblew999/charter/go/lib/cmd/workerbench -base http://localhost:5174 \
//	  -H 'Authorization: Bearer local-write-token' \
//	  'GET /api/hello' 'GET /api/notes' 'POST /api/notes {"body":"bench"}'
//
// A request is "METHOD PATH" and, for a POST, a body: JSON when it starts with { or [, a form
// otherwise. Requests go one at a time, so each costs what it costs alone. Each run sends warm
// requests of a kind first, not counted (a runtime's start, the first compile), then n counted ones;
// runs repeat, and the table has every run and their median, so how much a number moves is on the page.
// Without -pid it counts every process named workerd, so stop other Workers first.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

type request struct{ label, method, path, body string }

type headers []string

func (h *headers) String() string     { return strings.Join(*h, ", ") }
func (h *headers) Set(v string) error { *h = append(*h, v); return nil }

func main() {
	base := flag.String("base", os.Getenv("BASE"), "the Worker's URL")
	pid := flag.Int("pid", 0, "workerd's process id (0: every process named workerd)")
	n := flag.Int("n", 60, "counted requests per kind and run")
	warm := flag.Int("warm", 5, "uncounted requests per kind first")
	runs := flag.Int("runs", 3, "runs")
	label := flag.String("label", "", "what is measured, for the table's title")
	every := flag.Duration("every", 0, "at most one request per this long (to stay under a rate limit; waiting costs no CPU)")
	var hs headers
	flag.Var(&hs, "H", "a header for every request, 'Name: value' (repeatable)")
	flag.Parse()
	if *base == "" || flag.NArg() == 0 {
		fail("usage: workerbench -base URL [-H 'Name: value'] 'METHOD PATH [BODY]'…")
	}
	var requests []request
	for _, arg := range flag.Args() {
		method, rest, _ := strings.Cut(arg, " ")
		path, body, _ := strings.Cut(rest, " ")
		requests = append(requests, request{strings.TrimSpace(method + " " + path), method, path, body})
	}
	pids := []int{*pid}
	if *pid == 0 {
		if pids = workerds(); len(pids) == 0 {
			fail("no process named workerd: start the Worker first")
		}
	}
	results := make([][]float64, len(requests)) // ms of CPU per request: [kind][run]
	for run := 0; run < *runs; run++ {
		for k, rq := range requests {
			for i := 0; i < *warm; i++ {
				send(*base, hs, rq)
				time.Sleep(*every)
			}
			before := cpu(pids)
			for i := 0; i < *n; i++ {
				send(*base, hs, rq)
				time.Sleep(*every)
			}
			results[k] = append(results[k], float64(cpu(pids)-before)/float64(time.Millisecond)/float64(*n))
		}
	}
	title := *label
	if title == "" {
		title = *base
	}
	fmt.Printf("%s: workerd CPU per request, ms (%d runs × %d requests, after %d warm; %s/%s)\n", title, *runs, *n, *warm, runtime.GOOS, runtime.GOARCH)
	for k, rq := range requests {
		cells := make([]string, len(results[k]))
		for i, v := range results[k] {
			cells[i] = fmt.Sprintf("%.2f", v)
		}
		fmt.Printf("  %-34s median %7.2f   runs %s\n", rq.label, median(results[k]), strings.Join(cells, " "))
	}
}

// send makes one request and reads its whole answer. Anything but a 2xx stops the measurement, so a
// refused or broken request never looks cheap.
func send(base string, hs headers, rq request) {
	var body io.Reader
	if rq.body != "" {
		body = strings.NewReader(rq.body)
	}
	req, err := http.NewRequest(rq.method, base+rq.path, body)
	if err != nil {
		fail(err.Error())
	}
	if rq.body != "" {
		if strings.HasPrefix(rq.body, "{") || strings.HasPrefix(rq.body, "[") {
			req.Header.Set("Content-Type", "application/json")
		} else {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	for _, h := range hs {
		name, value, _ := strings.Cut(h, ":")
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fail(err.Error())
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode/100 != 2 {
		fail(fmt.Sprintf("%s: %d", rq.label, res.StatusCode))
	}
}

// workerds are the ids of the processes named workerd.
func workerds() []int {
	out, err := exec.Command("ps", "-A", "-o", "pid=,comm=").Output()
	if err != nil {
		fail(err.Error())
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && (f[1] == "workerd" || strings.HasSuffix(f[1], "/workerd")) {
			if id, err := strconv.Atoi(f[0]); err == nil {
				pids = append(pids, id)
			}
		}
	}
	return pids
}

// cpu is the processes' CPU time so far, user and system: from /proc on Linux, from ps elsewhere.
func cpu(pids []int) time.Duration {
	var total time.Duration
	for _, pid := range pids {
		if runtime.GOOS == "linux" {
			b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
			if err != nil {
				fail(err.Error())
			}
			s := string(b)
			f := strings.Fields(s[strings.LastIndexByte(s, ')')+2:]) // after the command name, which can hold spaces
			utime, _ := strconv.ParseInt(f[11], 10, 64)
			stime, _ := strconv.ParseInt(f[12], 10, 64)
			total += time.Duration(utime+stime) * time.Second / time.Duration(clockTicks())
			continue
		}
		out, err := exec.Command("ps", "-o", "cputime=", "-p", strconv.Itoa(pid)).Output()
		if err != nil {
			fail(err.Error())
		}
		total += parseCPUTime(strings.TrimSpace(string(out)))
	}
	return total
}

// parseCPUTime reads ps's cputime: [[dd-]hh:]mm:ss[.cc].
func parseCPUTime(s string) time.Duration {
	var d time.Duration
	if day, rest, ok := strings.Cut(s, "-"); ok {
		days, _ := strconv.Atoi(day)
		d, s = time.Duration(days)*24*time.Hour, rest
	}
	parts := strings.Split(s, ":")
	units := []time.Duration{time.Hour, time.Minute, time.Second}[3-len(parts):]
	for i, p := range parts {
		v, _ := strconv.ParseFloat(p, 64)
		d += time.Duration(v * float64(units[i]))
	}
	return d
}

var ticks int64

// clockTicks is the kernel's clock ticks per second, the unit of /proc/<pid>/stat.
func clockTicks() int64 {
	if ticks == 0 {
		ticks = 100
		if out, err := exec.Command("getconf", "CLK_TCK").Output(); err == nil {
			if v, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil && v > 0 {
				ticks = v
			}
		}
	}
	return ticks
}

func median(v []float64) float64 {
	s := slices.Clone(v)
	slices.Sort(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "workerbench:", msg)
	os.Exit(1)
}
