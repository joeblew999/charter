// Package cfapi is the small Cloudflare REST client shared by cfdeploy and cftail: bearer auth
// and the {success, errors, result} envelope.
package cfapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// DefaultBaseURL is the Cloudflare API.
const DefaultBaseURL = "https://api.cloudflare.com/client/v4"

// Client calls the API for one account.
type Client struct {
	Token     string
	AccountID string
	BaseURL   string       // default DefaultBaseURL
	HTTP      *http.Client // default http.DefaultClient
}

// Body is a request body with its content type.
type Body struct {
	ContentType string
	Data        []byte
}

// JSON encodes v as a request body.
func JSON(v any) *Body {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return &Body{ContentType: "application/json", Data: b}
}

// AccountPath prefixes p with /accounts/{AccountID}.
func (c *Client) AccountPath(p string) string { return "/accounts/" + c.AccountID + p }

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error is a failed API call: the HTTP status and Cloudflare's error list (empty for a non-JSON response).
type Error struct {
	Method, Path string
	Status       int
	Errors       []apiError
}

func (e *Error) Error() string {
	if e.Errors == nil {
		return fmt.Sprintf("%s %s: HTTP %d, non-JSON response", e.Method, e.Path, e.Status)
	}
	return fmt.Sprintf("%s %s: HTTP %d: %v", e.Method, e.Path, e.Status, e.Errors)
}

// IsNotFound reports whether err is an API answer that the resource doesn't exist: HTTP 404, or a Workers
// "not found" error code (10007 script, 10025 preview or deployment).
func IsNotFound(err error) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	if e.Status == http.StatusNotFound {
		return true
	}
	for _, x := range e.Errors {
		if x.Code == 10007 || x.Code == 10025 {
			return true
		}
	}
	return false
}

// Do calls method apiPath and decodes the envelope's result into out (if non-nil). bearer overrides the
// API token (asset uploads authenticate with an upload-session JWT).
func (c *Client) Do(ctx context.Context, method, apiPath, bearer string, body *Body, out any) error {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body.Data)
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, method, base+apiPath, r)
	if err != nil {
		return err
	}
	if bearer == "" {
		bearer = c.Token
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	if body != nil {
		req.Header.Set("Content-Type", body.ContentType)
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var env struct {
		Success bool            `json:"success"`
		Errors  []apiError      `json:"errors"`
		Result  json.RawMessage `json:"result"`
	}
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &env); err != nil {
		return &Error{Method: method, Path: apiPath, Status: resp.StatusCode}
	}
	if !env.Success {
		return &Error{Method: method, Path: apiPath, Status: resp.StatusCode, Errors: env.Errors}
	}
	if out != nil && len(env.Result) > 0 && string(env.Result) != "null" {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}
