package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/joeblew999/charter/internal/project"
)

// A deployed database keeps a copy in each of D1's regions, so a read is answered near its reader: read replication,
// which Cloudflare turns on for one database at a time, through its dashboard or its API and not through wrangler
// (https://developers.cloudflare.com/d1/best-practices/read-replication/#enable-read-replication-via-rest-api). So
// charter deploy turns it on, for every database the deployed Worker is bound to: no developer has to remember it,
// and a database made by this deploy has it from its first day.
//
// It is safe for every app because charter carries the bookmarks (cloud/session.go): an app that read copies and
// carried none would show a reader a page without what they had just written. The deploy that has put the code
// there is what turns the copies on, in that order.

// replicated turns read replication on for each D1 database the deployed Worker is bound to that has it off. It
// says what it did; what it could not do it says too, and the deploy stands: the Worker works without copies.
func replicated(p *project.Project, env string) {
	deployed, ok := deployedBindings(p, env)
	if !ok {
		return
	}
	token, account := cloudflareSecret(p, "CLOUDFLARE_API_TOKEN"), cloudflareSecret(p, "CLOUDFLARE_ACCOUNT_ID")
	for _, b := range deployed {
		if b.Kind != "d1" || token == "" || account == "" {
			continue
		}
		address := "https://api.cloudflare.com/client/v4/accounts/" + account + "/d1/database/" + b.To
		mode, err := replication(http.MethodGet, address, token, nil)
		if err == nil && mode != "auto" {
			mode, err = replication(http.MethodPut, address, token, map[string]any{"read_replication": map[string]string{"mode": "auto"}})
			if err == nil && mode == "auto" {
				say("the database %s now keeps a copy in each of D1's regions: a read is answered by the one nearest its reader (read replication)", b.Binding)
			}
		}
		if err != nil {
			say("· the database %s: read replication was not turned on (%v). The Worker works without it, every read going to the primary; the token needs D1: Edit", b.Binding, err)
		}
	}
}

// replication asks Cloudflare's API about one database, or changes it, and gives its read replication's mode.
func replication(method, address, token string, body any) (string, error) {
	var sent bytes.Buffer
	if body != nil {
		json.NewEncoder(&sent).Encode(body)
	}
	req, _ := http.NewRequest(method, address, &sent)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var answer struct {
		Success bool
		Errors  []struct{ Message string }
		Result  struct {
			ReadReplication struct{ Mode string } `json:"read_replication"`
		}
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil {
		return "", fmt.Errorf("Cloudflare's API: HTTP %d", res.StatusCode)
	}
	if !answer.Success {
		if len(answer.Errors) > 0 {
			return "", fmt.Errorf("Cloudflare's API: %s", answer.Errors[0].Message)
		}
		return "", fmt.Errorf("Cloudflare's API: HTTP %d", res.StatusCode)
	}
	return answer.Result.ReadReplication.Mode, nil
}
