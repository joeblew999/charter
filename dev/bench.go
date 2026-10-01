package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

func init() {
	commands["bench"] = command{"[-n 20] <url>", "wall time per route of a notes API, as a client sees it (docs/benchmarks.md)", bench}
}

// bench times the read routes of the notes API: median and slowest of n requests after 3 warm-up
// ones. CPU time, what Workers bills, is in Workers Logs, not here.
func bench(args []string) error {
	n := 20
	rest := flags("bench", args, func(f *flag.FlagSet) { f.IntVar(&n, "n", n, "requests per route") })
	if len(rest) != 1 || n < 1 {
		return errors.New("bench needs <url>, e.g. https://orpc-api-go.gedw99.workers.dev or http://localhost:5174")
	}
	base := strings.TrimSuffix(rest[0], "/")
	fmt.Printf("%s, %d requests per route, wall time\n", base, n)
	for _, path := range []string{"/api/hello", "/api/notes?limit=20", "/api/openapi.json", "/api/nope"} {
		var times []time.Duration
		status := 0
		for i := -3; i < n; i++ {
			start := time.Now()
			res, err := http.Get(base + path)
			if err != nil {
				return err
			}
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
			status = res.StatusCode
			if i >= 0 {
				times = append(times, time.Since(start))
			}
		}
		sort.Slice(times, func(a, b int) bool { return times[a] < times[b] })
		ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
		fmt.Printf("  %-24s HTTP %d  median %6.1f ms  slowest %6.1f ms\n", path, status, ms(times[len(times)/2]), ms(times[len(times)-1]))
	}
	return nil
}
