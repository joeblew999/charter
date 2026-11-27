package command

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/project"
)

// The deployed Worker's logs. Alone, charter run logs follows them as they come (wrangler tail), until Ctrl-C. With
// --since it reads what Cloudflare kept (Workers Logs, wrangler.jsonc's observability, seven days): what the Worker
// printed and how each request ended, newest last.
//
//	charter run logs [env]                        live, until Ctrl-C
//	charter run logs [env] --since 1h             the last hour, as kept
//	charter run logs [env] --since 1h --errors    only what went wrong: errors, exceptions, a 5xx
//	charter run logs [env] --since 1d --find ray  only lines that have this text (a request's reference)

func logs(p *project.Project, args []string) {
	env := takeEnv(p, &args)
	args = slices.Clone(args)
	i := slices.Index(args, "--since")
	if i < 0 {
		flags := []string{"tail"}
		if env != "" {
			flags = append(flags, "--env", env)
		}
		check(cloudflare(p, flags, args))
		return
	}
	if i+1 >= len(args) {
		check(errors.New("usage: charter run logs [env] --since 1h [--errors] [--find text]"))
	}
	since, err := span(args[i+1])
	if err != nil || since <= 0 {
		check(fmt.Errorf("--since %s: a span like 30m, 6h or 7d", args[i+1]))
	}
	args = slices.Delete(args, i, i+2)
	wrong, find := false, ""
	if i := slices.Index(args, "--errors"); i >= 0 {
		args, wrong = slices.Delete(args, i, i+1), true
	}
	if i := slices.Index(args, "--find"); i >= 0 && i+1 < len(args) {
		find = args[i+1]
		args = slices.Delete(args, i, i+2)
	}
	if len(args) > 0 {
		check(errors.New("usage: charter run logs [env] --since 1h [--errors] [--find text]"))
	}
	lines, err := keptLogs(p, deployedName(p, env), since, wrong, find)
	check(err)
	if len(lines) == 0 {
		say("nothing%s in the last %s (Workers Logs keep what the Worker printed once wrangler.jsonc has \"observability\": { \"enabled\": true })", map[bool]string{true: " went wrong"}[wrong], spanWords(since))
		return
	}
	for _, line := range lines {
		fmt.Println(line)
	}
}

// keptLogs are the Worker's log lines Cloudflare kept, oldest first.
func keptLogs(p *project.Project, worker string, since time.Duration, wrong bool, find string) ([]string, error) {
	token, account := cloudflareSecret(p, "CLOUDFLARE_API_TOKEN"), cloudflareSecret(p, "CLOUDFLARE_ACCOUNT_ID")
	if token == "" || account == "" {
		return nil, errors.New("Cloudflare's credentials are not in the environment or in fnox")
	}
	filter := func(key, operation, kind string, value any) map[string]any {
		return map[string]any{"key": key, "operation": operation, "type": kind, "value": value}
	}
	filters := []any{filter("$metadata.service", "eq", "string", worker)}
	if wrong {
		// What went wrong: a line printed as an error, a request Cloudflare ended, or one answered with a 5xx.
		filters = append(filters, map[string]any{"kind": "group", "filterCombination": "or", "filters": []any{
			filter("$metadata.level", "eq", "string", "error"),
			// A request Cloudflare ended. One whose reader left first (a stream that was closed) went as it should.
			map[string]any{"kind": "group", "filterCombination": "and", "filters": []any{
				filter("$workers.outcome", "exists", "string", ""),
				filter("$workers.outcome", "neq", "string", "ok"),
				filter("$workers.outcome", "neq", "string", "canceled"),
				filter("$workers.outcome", "neq", "string", "responseStreamDisconnected"),
			}},
			filter("$workers.event.response.status", "gte", "number", 500),
		}})
	}
	parameters := map[string]any{"datasets": []string{}, "filters": filters, "calculations": []any{}, "groupBys": []any{}, "havings": []any{}}
	if find != "" {
		parameters["needle"] = map[string]any{"value": find, "matchCase": false}
	}
	now := time.Now()
	body, _ := json.Marshal(map[string]any{
		"queryId": "charter-logs", "view": "events", "limit": 200, "dry": true, "parameters": parameters,
		"timeframe": map[string]int64{"from": now.Add(-since).UnixMilli(), "to": now.UnixMilli()},
	})
	req, _ := http.NewRequest(http.MethodPost, "https://api.cloudflare.com/client/v4/accounts/"+account+"/workers/observability/telemetry/query", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var answer struct {
		Success bool
		Errors  []struct{ Message string }
		Result  struct {
			Events struct {
				Events []struct {
					Timestamp int64 `json:"timestamp"`
					Metadata  struct {
						Level   string `json:"level"`
						Message string `json:"message"`
						Error   string `json:"error"`
					} `json:"$metadata"`
					Workers struct {
						Outcome string `json:"outcome"`
						Event   struct {
							Request  struct{ Method, URL string }
							Response struct{ Status int }
						} `json:"event"`
					} `json:"$workers"`
				} `json:"events"`
			} `json:"events"`
		}
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
		return nil, fmt.Errorf("Cloudflare's logs: HTTP %d: %w", res.StatusCode, err)
	}
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("Cloudflare's token may not read the Worker's logs: add the permission \"Workers Observability: Edit\" to it (https://dash.cloudflare.com/profile/api-tokens); reading logs is under that name")
	}
	if !answer.Success {
		why := fmt.Sprintf("HTTP %d", res.StatusCode)
		if len(answer.Errors) > 0 {
			why = answer.Errors[0].Message
		}
		return nil, errors.New("Cloudflare's logs: " + why)
	}
	var lines []string
	for _, e := range answer.Result.Events.Events {
		line := time.UnixMilli(e.Timestamp).Local().Format("01-02 15:04:05") + "  "
		if request := e.Workers.Event.Request; request.URL != "" {
			at := request.URL
			if i := strings.Index(at, "://"); i >= 0 {
				if j := strings.Index(at[i+3:], "/"); j >= 0 {
					at = at[i+3+j:]
				}
			}
			line += fmt.Sprintf("%s %s → %d", request.Method, at, e.Workers.Event.Response.Status)
			if e.Workers.Outcome != "" && e.Workers.Outcome != "ok" {
				line += " (" + e.Workers.Outcome + ")"
			}
			if e.Metadata.Message != "" && !strings.HasPrefix(e.Metadata.Message, request.Method+" ") {
				line += "  " + e.Metadata.Message
			}
		} else {
			line += e.Metadata.Message
		}
		if e.Metadata.Level == "error" || e.Metadata.Error != "" {
			line = "✗ " + line
			if e.Metadata.Error != "" && !strings.Contains(line, e.Metadata.Error) {
				line += "  " + e.Metadata.Error
			}
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}
	slices.Reverse(lines)
	return lines, nil
}
