// Package cflogs reads a deployed Worker's stored logs (Workers Logs), which every charter deploy turns on: what it
// logged before now, where a tail (package cftail) only shows what happens while it is watched.
//
// API: POST …/accounts/{id}/workers/observability/telemetry/query, the Workers Observability API
// (https://developers.cloudflare.com/workers/observability/query-builder/), with its "events" view: the request is
// the one Cloudflare's own observability MCP server sends (cloudflare/mcp-server-cloudflare,
// apps/workers-observability). Workers Logs keeps events for days, not for ever.
package cflogs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/cfapi"
)

// Dataset is where Workers Logs are.
const Dataset = "cloudflare-workers"

// MaxLimit is the most events one query returns.
const MaxLimit = 2000

// Query is which of a Worker's events are wanted.
type Query struct {
	Script string        // the Worker's name (required)
	Since  time.Duration // how far back from To (default 10 minutes)
	To     time.Time     // the end of the time asked for (default now)
	Level  string        // only this level: "error", "warn", "info", "log", "debug" ("" is all)
	Search string        // only events with this text
	Status int           // only requests answered with this status (0 is all)
	Limit  int           // at most this many, newest first (default 50, at most MaxLimit)
}

// Event is one stored event: a request the Worker served (its invocation log), or one of its console messages.
type Event struct {
	Time    time.Time
	Level   string // "info", "error", …
	Message string
	Method  string // of the request it belongs to, when there is one
	URL     string
	Status  int
	Outcome string // "ok", "exception", …
	Request string // the request's id: its events share it
}

// Client talks to the Cloudflare API for one account.
type Client struct {
	Token     string
	AccountID string
	BaseURL   string // default https://api.cloudflare.com/client/v4
}

type filter struct {
	Key       string `json:"key"`
	Operation string `json:"operation"`
	Type      string `json:"type"`
	Value     any    `json:"value"`
}

// request is q as the API takes it.
func request(q Query, now time.Time) (map[string]any, error) {
	if q.Script == "" {
		return nil, fmt.Errorf("cflogs: no Worker named")
	}
	to := q.To
	if to.IsZero() {
		to = now
	}
	since := q.Since
	if since <= 0 {
		since = 10 * time.Minute
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > MaxLimit {
		return nil, fmt.Errorf("cflogs: limit %d: at most %d events in one query", limit, MaxLimit)
	}
	filters := []filter{{"$metadata.service", "eq", "string", q.Script}}
	if q.Level != "" {
		filters = append(filters, filter{"$metadata.level", "eq", "string", q.Level})
	}
	if q.Status != 0 {
		filters = append(filters, filter{"$workers.event.response.status", "eq", "number", q.Status})
	}
	parameters := map[string]any{"datasets": []string{Dataset}, "filters": filters, "filterCombination": "and"}
	if q.Search != "" {
		parameters["needle"] = map[string]any{"value": q.Search, "matchCase": false}
	}
	return map[string]any{
		"queryId":    "charter-logs",
		"view":       "events",
		"limit":      limit,
		"dry":        true, // run the query without saving it in the account
		"timeframe":  map[string]int64{"from": to.Add(-since).UnixMilli(), "to": to.UnixMilli()},
		"parameters": parameters,
	}, nil
}

// returned is one event as the API returns it. source is the console message: an object with a message, or a string.
type returned struct {
	Timestamp int64           `json:"timestamp"`
	Source    json.RawMessage `json:"source"`
	Workers   struct {
		Outcome string `json:"outcome"`
		Event   struct {
			Request struct {
				Method string `json:"method"`
				URL    string `json:"url"`
			} `json:"request"`
			Response struct {
				Status int `json:"status"`
			} `json:"response"`
		} `json:"event"`
		RequestID string `json:"requestId"`
	} `json:"$workers"`
	Metadata struct {
		Level     string `json:"level"`
		Message   string `json:"message"`
		Error     string `json:"error"`
		RequestID string `json:"requestId"`
	} `json:"$metadata"`
}

func (r returned) event() Event {
	e := Event{Time: time.UnixMilli(r.Timestamp).UTC(), Level: r.Metadata.Level, Message: r.Metadata.Message,
		Method: r.Workers.Event.Request.Method, URL: r.Workers.Event.Request.URL, Status: r.Workers.Event.Response.Status,
		Outcome: r.Workers.Outcome, Request: r.Metadata.RequestID}
	if e.Request == "" {
		e.Request = r.Workers.RequestID
	}
	var source struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	var text string
	switch {
	case json.Unmarshal(r.Source, &text) == nil:
		source.Message = text
	default:
		json.Unmarshal(r.Source, &source)
	}
	if e.Message == "" {
		e.Message = source.Message
	}
	if e.Message == "" {
		e.Message = r.Metadata.Error
	}
	if e.Level == "" {
		e.Level = source.Level
	}
	return e
}

// Events are the Worker's stored events that q asks for, oldest first.
func (c *Client) Events(ctx context.Context, q Query) ([]Event, error) {
	body, err := request(q, time.Now())
	if err != nil {
		return nil, err
	}
	api := &cfapi.Client{Token: c.Token, AccountID: c.AccountID, BaseURL: c.BaseURL}
	var result struct {
		Events struct {
			Events []returned `json:"events"`
		} `json:"events"`
	}
	if err := api.Do(ctx, "POST", api.AccountPath("/workers/observability/telemetry/query"), "", cfapi.JSON(body), &result); err != nil {
		return nil, fmt.Errorf("cflogs: %w", err)
	}
	events := make([]Event, len(result.Events.Events))
	for i, r := range result.Events.Events {
		events[len(events)-1-i] = r.event() // the API returns the newest first
	}
	return events, nil
}

// Format is e as one line: its time, level, the request it belongs to, and the message.
func Format(e Event) string {
	var b strings.Builder
	b.WriteString(e.Time.Format("15:04:05.000"))
	level := e.Level
	if level == "" {
		level = "log"
	}
	fmt.Fprintf(&b, " %-5s", level)
	if e.Status != 0 {
		fmt.Fprintf(&b, " %d", e.Status)
	}
	if e.Outcome != "" && e.Outcome != "ok" {
		fmt.Fprintf(&b, " (%s)", e.Outcome)
	}
	// An invocation's own message is its method and URL: say a request once.
	if request := strings.TrimSpace(e.Method + " " + e.URL); request != "" && request != e.Message {
		b.WriteString(" " + request + ":")
	}
	b.WriteString(" " + e.Message)
	return b.String()
}
