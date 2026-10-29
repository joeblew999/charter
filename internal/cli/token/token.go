// Package token is charter token: whether CLOUDFLARE_API_TOKEN is an active token, before a deploy uses it.
package token

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// api is Cloudflare's API.
var api = "https://api.cloudflare.com/client/v4"

// Main runs the step with args, the arguments after its name.
func Main(args []string) {
	fs := flag.NewFlagSet("charter token", flag.ExitOnError)
	fs.Parse(args)
	log.SetFlags(0)
	if err := Check(os.Getenv("CLOUDFLARE_API_TOKEN"), os.Getenv("CLOUDFLARE_ACCOUNT_ID")); err != nil {
		log.Fatal("✗ ", err)
	}
	log.Print("✓ token active")
}

// Check verifies token with Cloudflare: set, with an account, and active.
func Check(token, account string) error {
	if token == "" || account == "" {
		return fmt.Errorf("CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID must be set (fnox exec --)")
	}
	req, err := http.NewRequest(http.MethodGet, api+"/user/tokens/verify", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var v struct {
		Success bool `json:"success"`
		Result  struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		return fmt.Errorf("Cloudflare's answer (HTTP %d): %v", res.StatusCode, err)
	}
	if !v.Success || v.Result.Status != "active" {
		return fmt.Errorf("the token is invalid or not active (HTTP %d, status %q)", res.StatusCode, v.Result.Status)
	}
	return nil
}
