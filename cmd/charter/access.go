package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

func init() {
	commands["access"] = command{"setup [email...] | token create <machine> <file|fnox> | token list | token revoke <machine> | delete",
		"REMOTE: Cloudflare Access in front of the Worker (API_URL): people log in with GitHub, each machine has a service token of its own. Needs CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID, CF_ACCESS_TEAM_DOMAIN and CF_ACCESS_GITHUB_IDP_ID (the environment, or fnox). Secrets are never printed", access}
}

// The Access application of a Worker is the one for its hostname. Its service tokens are named
// <worker>:<machine>; the IDs and the machine credentials go into fnox as <PREFIX>_ACCESS_..., where
// PREFIX is the Worker's name in capitals (go/auth's EnvPrefix, which the SDKs read).
func access(args []string) error {
	address, worker, err := deployed()
	if err != nil {
		return err
	}
	cf, err := newCloudflare()
	if err != nil {
		return err
	}
	a := accessApp{cf: cf, host: address.Host, worker: worker, prefix: envPrefix(worker)}
	switch strings.Join(args[:min(len(args), 2)], " ") {
	case "token create":
		if len(args) != 4 {
			return errors.New("access token create <machine> <file|fnox>")
		}
		return a.create(args[2], args[3])
	case "token list":
		return a.list()
	case "token revoke":
		if len(args) != 3 {
			return errors.New("access token revoke <machine>")
		}
		return a.revoke(args[2])
	}
	switch {
	case len(args) > 0 && args[0] == "setup":
		return a.setup(args[1:])
	case len(args) == 1 && args[0] == "delete":
		return a.delete()
	}
	return errors.New("access setup [email...] | token create <machine> <file|fnox> | token list | token revoke <machine> | delete")
}

// envPrefix is go/auth's EnvPrefix: the start of the variables an API's SDKs read.
func envPrefix(name string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(name))
}

// cloudflare calls Cloudflare's API for one account.
type cloudflare struct{ token, account string }

func newCloudflare() (cloudflare, error) {
	cf := cloudflare{secret("CLOUDFLARE_API_TOKEN"), secret("CLOUDFLARE_ACCOUNT_ID")}
	if cf.token == "" || cf.account == "" {
		return cf, errors.New("CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID are not set (in the environment, or in fnox)")
	}
	return cf, nil
}

var ids = regexp.MustCompile(`[0-9a-f-]{32,}`)

// call sends body (if any) to path under the account and decodes the result into out (if any).
// What it says on failure has no IDs in it.
func (cf cloudflare) call(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "https://api.cloudflare.com/client/v4/accounts/"+cf.account+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cf.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var answer struct {
		Success bool            `json:"success"`
		Errors  json.RawMessage `json:"errors"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil || !answer.Success {
		return fmt.Errorf("%s %s: HTTP %d %s", method, ids.ReplaceAllString(path, "<id>"), res.StatusCode, answer.Errors)
	}
	if out != nil {
		return json.Unmarshal(answer.Result, out)
	}
	return nil
}

type accessApp struct {
	cf                   cloudflare
	host, worker, prefix string
}

type app struct {
	ID       string `json:"id"`
	AUD      string `json:"aud"`
	Domain   string `json:"domain"`
	Policies []struct {
		Decision string `json:"decision"`
		Include  []struct {
			Email *struct {
				Email string `json:"email"`
			} `json:"email"`
		} `json:"include"`
	} `json:"policies"`
}

type serviceToken struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	ExpiresAt    string `json:"expires_at"`
}

// app is the Access application for the Worker's hostname, or nil.
func (a accessApp) app() (*app, error) {
	var apps []app
	if err := a.cf.call("GET", "/access/apps?per_page=1000", nil, &apps); err != nil {
		return nil, err
	}
	for i := range apps {
		if apps[i].Domain == a.host {
			return &apps[i], nil
		}
	}
	return nil, nil
}

// tokens are the Worker's machines' service tokens.
func (a accessApp) tokens() ([]serviceToken, error) {
	var all []serviceToken
	if err := a.cf.call("GET", "/access/service_tokens?per_page=1000", nil, &all); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(t serviceToken) bool { return !strings.HasPrefix(t.Name, a.worker+":") }), nil
}

// put creates or updates the application: the machines' service tokens pass (Service Auth), and
// the people in emails log in with GitHub. A refused program gets 401, not the login page.
func (a accessApp) put(existing *app, emails []string) (*app, error) {
	team, idp := secret("CF_ACCESS_TEAM_DOMAIN"), secret("CF_ACCESS_GITHUB_IDP_ID")
	if team == "" || idp == "" {
		return nil, errors.New("CF_ACCESS_TEAM_DOMAIN and CF_ACCESS_GITHUB_IDP_ID are not set (in the environment, or in fnox)")
	}
	tokens, err := a.tokens()
	if err != nil {
		return nil, err
	}
	var policies []map[string]any
	if len(tokens) > 0 {
		var include []map[string]any
		for _, t := range tokens {
			include = append(include, map[string]any{"service_token": map[string]string{"token_id": t.ID}})
		}
		policies = append(policies, map[string]any{"name": a.worker + " machines", "decision": "non_identity", "include": include})
	}
	if len(emails) > 0 {
		var include []map[string]any
		for _, e := range emails {
			include = append(include, map[string]any{"email": map[string]string{"email": e}})
		}
		policies = append(policies, map[string]any{
			"name": a.worker + " people", "decision": "allow", "include": include,
			"require": []map[string]any{{"login_method": map[string]string{"id": idp}}},
		})
	}
	for i := range policies {
		policies[i]["precedence"] = i + 1
	}
	body := map[string]any{
		"name": a.worker, "type": "self_hosted", "domain": a.host,
		"destinations": []map[string]string{{"type": "public", "uri": a.host}},
		"allowed_idps": []string{idp}, "auto_redirect_to_identity": true, "app_launcher_visible": false,
		"session_duration": "24h", "http_only_cookie_attribute": true, "service_auth_401_redirect": true,
		"policies": policies,
	}
	var result app
	if existing != nil {
		err = a.cf.call("PUT", "/access/apps/"+existing.ID, body, &result)
	} else {
		err = a.cf.call("POST", "/access/apps", body, &result)
	}
	return &result, err
}

// emails are the people an application lets log in.
func (x *app) emails() []string {
	var out []string
	if x == nil {
		return nil
	}
	for _, p := range x.Policies {
		for _, i := range p.Include {
			if p.Decision == "allow" && i.Email != nil {
				out = append(out, i.Email.Email)
			}
		}
	}
	return out
}

// setup puts the application in front of the Worker, and gives the Worker the settings go/auth
// reads, ACCESS_TEAM_DOMAIN and ACCESS_AUD: at once, and in fnox for every deploy after (the
// project's WORKER_OPTIONAL_SECRETS), with the application's ID.
func (a accessApp) setup(emails []string) error {
	existing, err := a.app()
	if err != nil {
		return err
	}
	if len(emails) == 0 {
		emails = existing.emails()
	}
	if len(emails) == 0 {
		return errors.New("access setup <email>...: who may log in with GitHub")
	}
	result, err := a.put(existing, emails)
	if err != nil {
		return err
	}
	fmt.Printf("the Access application for %s: %s, %d people may log in\n", a.host, map[bool]string{true: "updated", false: "created"}[existing != nil], len(emails))
	settings := [][2]string{{"ACCESS_TEAM_DOMAIN", secret("CF_ACCESS_TEAM_DOMAIN")}, {"ACCESS_AUD", result.AUD}}
	for _, s := range settings {
		if err := a.cf.call("PUT", "/workers/scripts/"+a.worker+"/secrets", map[string]string{"name": s[0], "text": s[1], "type": "secret_text"}, nil); err != nil {
			return fmt.Errorf("the Worker's secret %s (deploy the Worker first): %w", s[0], err)
		}
		fmt.Printf("set the Worker's secret %s\n", s[0])
	}
	for _, s := range append(settings, [2]string{"ACCESS_APP_ID", result.ID}) {
		if err := a.fnoxSet(s[0], s[1], "the Access application of "+a.worker); err != nil {
			return err
		}
	}
	return nil
}

// fnoxSet stores a value in the keychain through fnox, under a name in the project's fnox.toml,
// through a pipe: never on a command line.
func (a accessApp) fnoxSet(name, value, what string) error {
	cmd := exec.Command("fnox", "set", name, "-p", "keychain", "-k", a.worker+" "+name, "-d", what, "--if-missing", "ignore")
	cmd.Stdin, cmd.Stderr = strings.NewReader(value), os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fnox set %s: %w", name, err)
	}
	fmt.Println("fnox:", name)
	return nil
}

var machineName = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

// create makes a machine's service token, valid for a year, and lets it through the application.
// Its Client ID and Secret go into a file only its owner can read, or into fnox.
func (a accessApp) create(machine, into string) error {
	if !machineName.MatchString(machine) {
		return errors.New("a machine's name is lower-case letters, digits and dashes")
	}
	existing, err := a.app()
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("no Access application for " + a.host + ": run access setup first")
	}
	name := a.worker + ":" + machine
	tokens, err := a.tokens()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(tokens, func(t serviceToken) bool { return t.Name == name }) {
		return fmt.Errorf("%s exists: revoke it first, or pick another name", name)
	}
	var token serviceToken
	if err := a.cf.call("POST", "/access/service_tokens", map[string]string{"name": name, "duration": "8760h"}, &token); err != nil {
		return err
	}
	if into == "fnox" {
		if err := a.fnoxSet(a.prefix+"_ACCESS_CLIENT_ID", token.ClientID, "the service token "+name); err != nil {
			return err
		}
		if err := a.fnoxSet(a.prefix+"_ACCESS_CLIENT_SECRET", token.ClientSecret, "the service token "+name); err != nil {
			return err
		}
	} else {
		credentials, _ := json.Marshal(map[string]string{"client_id": token.ClientID, "client_secret": token.ClientSecret})
		if err := os.WriteFile(into, append(credentials, '\n'), 0o600); err != nil {
			return err
		}
		if err := os.Chmod(into, 0o600); err != nil {
			return err
		}
	}
	if _, err := a.put(existing, existing.emails()); err != nil {
		return err
	}
	fmt.Printf("%s: expires %s; its Client ID and Secret are in %s\n", name, token.ExpiresAt, into)
	return nil
}

func (a accessApp) list() error {
	tokens, err := a.tokens()
	if err != nil {
		return err
	}
	for _, t := range tokens {
		fmt.Printf("%s\texpires %s\n", strings.TrimPrefix(t.Name, a.worker+":"), t.ExpiresAt)
	}
	return nil
}

// revoke deletes a machine's token: Access refuses it from then on, and the others are untouched.
func (a accessApp) revoke(machine string) error {
	tokens, err := a.tokens()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(tokens, func(t serviceToken) bool { return t.Name == a.worker+":"+machine })
	if i < 0 {
		return fmt.Errorf("no service token %s:%s", a.worker, machine)
	}
	existing, err := a.app()
	if err != nil {
		return err
	}
	if err := a.cf.call("DELETE", "/access/service_tokens/"+tokens[i].ID, nil, nil); err != nil {
		return err
	}
	if existing != nil {
		if _, err := a.put(existing, existing.emails()); err != nil {
			return err
		}
	}
	fmt.Printf("%s:%s revoked\n", a.worker, machine)
	return nil
}

// delete removes the application and the machines' tokens: the Worker is open again, to whatever
// its own auth allows.
func (a accessApp) delete() error {
	existing, err := a.app()
	if err != nil {
		return err
	}
	if existing != nil {
		if err := a.cf.call("DELETE", "/access/apps/"+existing.ID, nil, nil); err != nil {
			return err
		}
		fmt.Println("deleted the Access application for", a.host)
	}
	tokens, err := a.tokens()
	if err != nil {
		return err
	}
	for _, t := range tokens {
		if err := a.cf.call("DELETE", "/access/service_tokens/"+t.ID, nil, nil); err != nil {
			return err
		}
		fmt.Println("deleted the service token", t.Name)
	}
	return nil
}
