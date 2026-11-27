package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/tools"
)

// How the deployed Worker is doing, from Cloudflare's own count of how every request ended. A request the Worker
// answers is a success there whatever it answered; what this is for is the requests it never got to answer: the
// ones Cloudflare ended because the Worker went over its memory or its CPU time, or because its code threw. Those
// are a 5xx to the reader, nothing in the app's own code knows of them, and wrangler dev has no such limits, so
// they are found here or not at all.
//
//	charter run health [env]             the last 24 hours
//	charter run health [env] --since 1h  another span: 30m, 6h, 7d
//
// charter deploy and charter test --deployed each say first how the Worker did before them (the last day, the
// last hour): what an earlier run or real use did to it is seen then, at no cost of time. A test run also asks,
// when it is done, for what Cloudflare has counted of it by then, and fails on a fault among those. A load waits
// for all of it. And once a day the same is asked for every app (.github/workflows/health.yml).

// outcome is how requests ended, as Cloudflare names it, and how many did.
type outcome struct {
	Status   string
	Requests int64
}

// noFault are the ends that are no fault: answered, or the reader left first (a stream that was closed).
var noFault = []string{"success", "clientDisconnected", "canceled", "responseStreamDisconnected"}

// said is what an end means, for the ones worth explaining.
var said = map[string]string{
	"exceededMemory":       "ended because the Worker went over its 128 MB of memory: every request it was answering then got a 503. Something holds memory for long (a request that lasts, a Go runtime per stream) or too much at once",
	"exceededCpu":          "ended because they took more CPU time than the Worker may: a handler that computes for too long",
	"scriptThrewException": "ended because the Worker's code threw: go tool charter run logs --since 1h --errors says what",
	"internalError":        "ended by an error inside Cloudflare, not the Worker",
}

// cloudflareSecret is one of Cloudflare's two credentials: the environment's (CI), else fnox's, read in the app's
// folder. It is handed on, never shown.
func cloudflareSecret(p *project.Project, name string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	fnox, err := tools.Fnox()
	if err != nil {
		return ""
	}
	cmd := exec.Command(fnox, "get", name)
	cmd.Dir = p.Root
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// health is the Worker's requests between two times, by how they ended, and the CPU time of the noFault ones (the
// median and the 99th percentile, in milliseconds).
func health(p *project.Project, worker string, from, to time.Time) (outcomes []outcome, cpu50, cpu99 float64, err error) {
	token, account := cloudflareSecret(p, "CLOUDFLARE_API_TOKEN"), cloudflareSecret(p, "CLOUDFLARE_ACCOUNT_ID")
	if token == "" || account == "" {
		return nil, 0, 0, errors.New("Cloudflare's credentials are not in the environment or in fnox")
	}
	query := `query($account: String!, $script: String!, $from: Time!, $to: Time!) { viewer { accounts(filter: {accountTag: $account}) {
		workersInvocationsAdaptive(limit: 1000, filter: {scriptName: $script, datetime_geq: $from, datetime_leq: $to}) {
			dimensions { status } sum { requests } quantiles { cpuTimeP50 cpuTimeP99 } } } } }`
	body, _ := json.Marshal(map[string]any{"query": query, "variables": map[string]string{
		"account": account, "script": worker, "from": from.UTC().Format(time.RFC3339), "to": to.UTC().Format(time.RFC3339)}})
	req, _ := http.NewRequest(http.MethodPost, "https://api.cloudflare.com/client/v4/graphql", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer res.Body.Close()
	var answer struct {
		Data struct {
			Viewer struct {
				Accounts []struct {
					Rows []struct {
						Dimensions struct{ Status string }
						Sum        struct{ Requests int64 }
						Quantiles  struct{ CPUTimeP50, CPUTimeP99 float64 }
					} `json:"workersInvocationsAdaptive"`
				}
			}
		}
		Errors []struct{ Message string }
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
		return nil, 0, 0, fmt.Errorf("Cloudflare's analytics: HTTP %d: %w", res.StatusCode, err)
	}
	if len(answer.Errors) > 0 {
		return nil, 0, 0, fmt.Errorf("Cloudflare's analytics: %s (the token needs Account Analytics: Read)", answer.Errors[0].Message)
	}
	if len(answer.Data.Viewer.Accounts) == 0 {
		return nil, 0, 0, nil
	}
	by := map[string]int64{}
	for _, row := range answer.Data.Viewer.Accounts[0].Rows {
		by[row.Dimensions.Status] += row.Sum.Requests
		if row.Dimensions.Status == "success" {
			// Microseconds, as Cloudflare counts them.
			cpu50, cpu99 = row.Quantiles.CPUTimeP50/1000, row.Quantiles.CPUTimeP99/1000
		}
	}
	for status, n := range by {
		outcomes = append(outcomes, outcome{status, n})
	}
	slices.SortFunc(outcomes, func(a, b outcome) int { return int(b.Requests - a.Requests) })
	return outcomes, cpu50, cpu99, nil
}

// unwell are the ends that are a fault, and how many requests there were in all.
func unwell(outcomes []outcome) (bad []outcome, total int64) {
	for _, o := range outcomes {
		total += o.Requests
		if !slices.Contains(noFault, o.Status) {
			bad = append(bad, o)
		}
	}
	return bad, total
}

// deployedName is the Worker's name on Cloudflare, for the Worker itself or an environment of it.
func deployedName(p *project.Project, env string) string {
	if env == "" {
		return p.Worker.Name
	}
	return p.Worker.Name + "-" + env
}

// report says how the Worker did in a span, and gives the ends that are a fault.
func report(worker, span string, outcomes []outcome, cpu50, cpu99 float64) []outcome {
	bad, total := unwell(outcomes)
	if total == 0 {
		say("%s had no requests %s", worker, span)
		return nil
	}
	say("%s, %s: %s requests", worker, span, grouped(total))
	for _, o := range bad {
		what := said[o.Status]
		if what == "" {
			what = "ended as Cloudflare's " + o.Status
		}
		say("✗ %s (%.1f%%) %s", grouped(o.Requests), 100*float64(o.Requests)/float64(total), what)
	}
	if len(bad) == 0 {
		say("✓ every one was answered: none went over its memory or CPU time, none threw")
	}
	for _, o := range outcomes {
		if o.Status == "clientDisconnected" {
			say("· %s readers left before their answer was done (a stream that was closed: usual)", grouped(o.Requests))
		}
	}
	if cpu99 > 0 {
		say("· CPU time: half under %.0f ms, 99%% under %.0f ms", cpu50, cpu99)
	}
	return bad
}

// grouped is a count with its thousands apart.
func grouped(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// span is a duration as a person writes one: 30m, 6h, 7d.
func span(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		return time.Duration(n) * 24 * time.Hour, err
	}
	return time.ParseDuration(s)
}

// spanWords is a duration as it is said: 30 minutes, 6 hours, 7 days.
func spanWords(d time.Duration) string {
	count := func(n time.Duration, one string) string {
		if n == 1 {
			return one
		}
		return fmt.Sprintf("%d %ss", n, one)
	}
	switch {
	case d%(24*time.Hour) == 0:
		return count(d/(24*time.Hour), "day")
	case d%time.Hour == 0:
		return count(d/time.Hour, "hour")
	}
	return count(d/time.Minute, "minute")
}

func healthTask(p *project.Project, args []string) {
	env := takeEnv(p, &args)
	since := 24 * time.Hour
	if i := slices.Index(args, "--since"); i >= 0 && i+1 < len(args) {
		d, err := span(args[i+1])
		if err != nil || d <= 0 {
			check(fmt.Errorf("--since %s: a span like 30m, 6h or 7d", args[i+1]))
		}
		since, args = d, slices.Delete(slices.Clone(args), i, i+2)
	}
	if len(args) > 0 {
		check(errors.New("usage: charter run health [env] [--since 6h]"))
	}
	now := time.Now()
	outcomes, cpu50, cpu99, err := health(p, deployedName(p, env), now.Add(-since), now)
	check(err)
	if bad := report(deployedName(p, env), "the last "+spanWords(since), outcomes, cpu50, cpu99); len(bad) > 0 {
		check(errors.New("the Worker is ending requests it should answer"))
	}
}

// healthOf says how the Worker did since a time, for a command that ran in it, and gives an error when a request
// ended in a fault. Cloudflare counts requests as they come to it, over the minute after they ended. So:
//
//   - sent 0 (a test run, which no one should wait on): it is asked once, at once, for what is counted by now. A
//     fault among those fails the run; the rest is counted within the minute, and the next deploy, the next test
//     and the daily check each say how the Worker did before them.
//   - sent > 0 (a load, which is asked for and takes its time): it waits until what was sent is counted, a minute
//     and a quarter at most, so that what it says is of all of it.
//
// Without credentials, or when Cloudflare cannot be asked, it says so and is no failure.
func healthOf(p *project.Project, env string, from time.Time, sent int64) error {
	worker := deployedName(p, env)
	var outcomes []outcome
	var cpu50, cpu99 float64
	for waited := time.Duration(0); ; waited += 5 * time.Second {
		var err error
		if outcomes, cpu50, cpu99, err = health(p, worker, from, time.Now()); err != nil {
			say("· how the Worker did meanwhile is not known: %v", err)
			return nil
		}
		if _, total := unwell(outcomes); sent == 0 || total >= sent*95/100 || waited >= 75*time.Second {
			break
		}
		time.Sleep(5 * time.Second)
	}
	what := "while it was loaded"
	if sent == 0 {
		what = "while it was tested, of what Cloudflare has counted so far"
	}
	if bad := report(worker, what, outcomes, cpu50, cpu99); len(bad) > 0 {
		return errors.New("requests of this run were ended by Cloudflare, not answered: the tests passing does not make that well")
	}
	if sent == 0 {
		say("· the rest is counted within the minute: go tool charter run health --since 10m, or the next deploy, says")
	}
	return nil
}
