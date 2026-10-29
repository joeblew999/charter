// Package cftail streams a deployed Worker's live logs (requests, console output, exceptions) without
// wrangler: it starts a tail with the REST API, reads its WebSocket, and deletes the tail when done.
//
// API: https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/tail/
// (POST …/workers/scripts/{name}/tails → {id, url, expires_at}; DELETE …/tails/{id}). Event shape:
// https://developers.cloudflare.com/workers/observability/logs/real-time-logs/. The WebSocket subprotocol
// ("trace-v1") isn't in the docs; it is what wrangler's tail uses. Cloudflare allows 10 tail clients per
// Worker and samples busy Workers.
package cftail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/joeblew999/charter/internal/cfapi"
)

// Subprotocol is the tail WebSocket protocol.
const Subprotocol = "trace-v1"

// Event is one Worker invocation as the tail reports it.
type Event struct {
	Outcome        string      `json:"outcome"` // "ok", "exception", "exceededCpu", "canceled", …
	ScriptName     string      `json:"scriptName"`
	EventTimestamp int64       `json:"eventTimestamp"` // Unix milliseconds
	Exceptions     []Exception `json:"exceptions"`
	Logs           []Log       `json:"logs"`
	Event          struct {
		Request *struct {
			URL    string `json:"url"`
			Method string `json:"method"`
		} `json:"request"`
		Response *struct {
			Status int `json:"status"`
		} `json:"response"`
		Cron string `json:"cron"`
	} `json:"event"`
}

// Exception is an uncaught exception.
type Exception struct {
	Name      string `json:"name"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// Log is one console call; Message holds its arguments.
type Log struct {
	Message   []any  `json:"message"`
	Level     string `json:"level"`
	Timestamp int64  `json:"timestamp"`
}

// Session is a started tail.
type Session struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

// Client talks to the Cloudflare API for one account.
type Client struct {
	Token     string
	AccountID string
	BaseURL   string // default https://api.cloudflare.com/client/v4
	Ready     func() // called once the tail WebSocket is connected; Cloudflare starts streaming a second or two later
}

func (c *Client) api() *cfapi.Client {
	return &cfapi.Client{Token: c.Token, AccountID: c.AccountID, BaseURL: c.BaseURL}
}

// Start creates a tail on script.
func (c *Client) Start(ctx context.Context, script string) (Session, error) {
	var s Session
	err := c.api().Do(ctx, "POST", c.api().AccountPath("/workers/scripts/"+script+"/tails"), "", cfapi.JSON(map[string]any{}), &s)
	if err == nil && (s.ID == "" || s.URL == "") {
		err = errors.New("cftail: start returned no id or url")
	}
	return s, err
}

// Delete removes a tail.
func (c *Client) Delete(ctx context.Context, script, id string) error {
	return c.api().Do(ctx, "DELETE", c.api().AccountPath("/workers/scripts/"+script+"/tails/"+id), "", nil, nil)
}

// Tail starts a tail on script, calls fn for every event until ctx is done or the stream ends, then
// deletes the tail.
func (c *Client) Tail(ctx context.Context, script string, fn func(Event)) error {
	s, err := c.Start(ctx, script)
	if err != nil {
		return err
	}
	defer func() {
		// The caller's context is usually cancelled by now (Ctrl-C): clean up with a fresh one.
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c.Delete(cleanup, script, s.ID)
	}()
	return stream(ctx, s.URL, c.Ready, fn)
}

// Stream reads events from a tail WebSocket URL until ctx is done (nil error) or the stream ends.
func Stream(ctx context.Context, url string, fn func(Event)) error { return stream(ctx, url, nil, fn) }

func stream(ctx context.Context, url string, ready func(), fn func(Event)) error {
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{Subprotocols: []string{Subprotocol}})
	if err != nil {
		return fmt.Errorf("cftail: dial: %w", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(32 << 20) // an event carries its console output: well over the 32 KB default
	if ready != nil {
		ready()
	}
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("cftail: stream: %w", err)
		}
		var e Event
		if err := json.Unmarshal(raw, &e); err != nil {
			return fmt.Errorf("cftail: bad event %.200s: %w", raw, err)
		}
		fn(e)
	}
}

// Format renders e as one summary line plus one indented line per log and exception.
func Format(e Event) string {
	var b strings.Builder
	b.WriteString(time.UnixMilli(e.EventTimestamp).UTC().Format("15:04:05.000"))
	switch r := e.Event.Request; {
	case r != nil:
		fmt.Fprintf(&b, " %s %s", r.Method, r.URL)
		if e.Event.Response != nil {
			fmt.Fprintf(&b, " %d", e.Event.Response.Status)
		}
	case e.Event.Cron != "":
		fmt.Fprintf(&b, " cron %s", e.Event.Cron)
	}
	fmt.Fprintf(&b, " (%s)", e.Outcome)
	for _, l := range e.Logs {
		parts := make([]string, len(l.Message))
		for i, m := range l.Message {
			if s, ok := m.(string); ok {
				parts[i] = s
			} else {
				j, _ := json.Marshal(m)
				parts[i] = string(j)
			}
		}
		fmt.Fprintf(&b, "\n  %s: %s", l.Level, strings.Join(parts, " "))
	}
	for _, x := range e.Exceptions {
		fmt.Fprintf(&b, "\n  exception %s: %s", x.Name, x.Message)
	}
	return b.String()
}
