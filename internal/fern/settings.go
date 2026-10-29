// Package fern is the SDKs, made with Fern from a project's fern/ (its specs and generators.yml): Fern's settings
// written from the spec and charter.toml's [sdks]; a group generated (Docker) into sdk/out/<group> when fern/ is newer
// than what made it; proven in its language (Go: build, vet, and Fern's tests against WireMock; TypeScript:
// typechecked with the TypeScript its SDKs are written for; Python: mypy and its own tests; the CLI, in Rust: built);
// and the Go SDK a project commits, in sdk/go. The toolchains a proof needs are mise's, at the versions pinned here.
package fern

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/joeblew999/charter/project"
)

// generator is a Fern generator, at the version charter pins: here, the one place.
type generator struct{ name, version string }

var generators = map[string]generator{
	"go":         {"fernapi/fern-go-sdk", "1.70.5"},
	"typescript": {"fernapi/fern-typescript-sdk", "3.106.1"},
	"python":     {"fernapi/fern-python-sdk", "5.41.2"},
	"cli":        {"fernapi/fern-cli-generator", "0.54.0"},
}

// scheme is a security scheme as the spec declares it: what Fern's settings restate of it.
type scheme struct {
	Type   string `json:"type"`
	Scheme string `json:"scheme"`
	Name   string `json:"name"`
	Header *struct {
		Name string `json:"name"`
		Env  string `json:"env"`
	} `json:"x-fern-header"`
	Flows *struct {
		ClientCredentials *struct {
			TokenURL string `json:"tokenUrl"`
		} `json:"clientCredentials"`
	} `json:"flows"`
}

// Settings is fern/generators.yml for these specs and this project: its SDKs, the Go SDK's module (goModule), and
// whether fern/ has an overlay of the OpenAPI spec (overlays.yml).
func Settings(specJSON []byte, sdks project.SDKs, goModule string, overlays bool) (string, error) {
	lines := []string{
		"# Written by `charter spec` from fern/openapi.json and charter.toml ([sdks]): don't edit, change",
		"# those and run charter spec. Fern's generators are charter's, at the versions it pins.",
		"api:",
		"  specs:",
		"    - openapi: openapi.json",
	}
	if overlays {
		lines = append(lines, "      overlays: overlays.yml")
	}
	lines = append(lines, "    - asyncapi: asyncapi.json")

	// Each scheme of the spec, restated: Fern sends only the credentials its settings name, and keeps only the first
	// scheme otherwise. OpenID Connect's token is not one: it goes as the bearer token. Any of them, each sent on
	// every request when given (Cloudflare Access wants its token on every request, even one the contract calls
	// public).
	// Upstream: fern-api/fern#17997 (when fixed: route credentials per operation, endpoint-security)
	var spec struct {
		Components struct {
			SecuritySchemes json.RawMessage `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return "", fmt.Errorf("fern/openapi.json: %w", err)
	}
	type named struct {
		name string
		scheme
	}
	var declared []named
	for _, kv := range ordered(spec.Components.SecuritySchemes) {
		var s scheme
		if err := json.Unmarshal(kv.value, &s); err != nil {
			return "", fmt.Errorf("security scheme %s: %w", kv.key, err)
		}
		declared = append(declared, named{kv.key, s})
	}
	kind := func(s scheme) int { return slices.Index([]string{"oauth2", "http", "apiKey"}, s.Type) }
	var schemes []named
	for _, d := range declared {
		if kind(d.scheme) >= 0 {
			schemes = append(schemes, d)
		}
	}
	slices.SortStableFunc(schemes, func(a, b named) int { return kind(a.scheme) - kind(b.scheme) })
	if len(schemes) > 0 {
		// An OAuth client's variables have the API's prefix, the one its access token's have.
		prefix := strings.ToUpper(sdks.Name) + "_"
		for _, d := range declared {
			if d.Header != nil && strings.HasSuffix(d.Header.Env, "_ACCESS_CLIENT_ID") {
				prefix = strings.TrimSuffix(d.Header.Env, "ACCESS_CLIENT_ID")
				break
			}
		}
		lines = append(lines, "  auth-schemes:")
		for _, d := range schemes {
			lines = append(lines, "    "+d.name+":")
			switch {
			case d.Type == "http" && d.Scheme == "bearer":
				lines = append(lines, "      scheme: bearer", "      token:", "        name: token")
			case d.Type == "apiKey":
				name := d.name
				if d.Header != nil && d.Header.Name != "" {
					name = d.Header.Name
				}
				lines = append(lines, "      header: "+d.Name, "      name: "+name)
				if d.Header != nil && d.Header.Env != "" {
					lines = append(lines, "      env: "+d.Header.Env)
				}
			case d.Type == "oauth2":
				if d.Flows == nil || d.Flows.ClientCredentials == nil || d.Flows.ClientCredentials.TokenURL == "" {
					return "", fmt.Errorf("the spec's OAuth scheme %s has no client credentials flow with a tokenUrl: the only OAuth flow the SDKs take", d.name)
				}
				lines = append(lines,
					"      scheme: oauth",
					"      type: client-credentials",
					"      client-id-env: "+prefix+"CLIENT_ID",
					"      client-secret-env: "+prefix+"CLIENT_SECRET",
					"      get-token:",
					`        endpoint: "POST `+d.Flows.ClientCredentials.TokenURL+`"`,
					"        request-properties:",
					"          client-id: $request.client_id",
					"          client-secret: $request.client_secret",
					"        response-properties:",
					"          access-token: $response.access_token",
					"          expires-in: $response.expires_in",
				)
			default:
				return "", fmt.Errorf("the spec's scheme %s is %s %s: the SDKs take bearer tokens, header keys and OAuth client credentials", d.name, d.Type, d.Scheme)
			}
		}
		lines = append(lines, "  auth:", "    any:")
		for _, d := range schemes {
			lines = append(lines, "      - "+d.name)
		}
	}

	lower := strings.ToLower(sdks.Name)
	group := func(name, about string, g generator, config []string, extra ...string) []string {
		out := []string{"", "  # " + about, "  " + name + ":"}
		out = append(out, extra...)
		out = append(out, "    generators:", "      - name: "+g.name, "        version: "+g.version, "        output:",
			"          location: local-file-system", "          path: ../sdk/out/"+name)
		if len(config) > 0 {
			out = append(out, "        config:")
			for _, c := range config {
				out = append(out, "          "+c)
			}
		}
		return out
	}
	typescript := []string{"namespaceExport: " + sdks.Name, "generateWebSocketClients: true",
		"# Workers (without nodejs_compat) have no `process`: the SDK's lookups of it are guarded.", "guardProcessEnvAccess: true"}
	lines = append(lines, "", "# The SDKs. Generate one: charter sdk <group>   (into sdk/out/<group>)", "groups:")
	lines = append(lines, group("go", "Go: the module charter sdk -publish commits in sdk/go, in a project that keeps it.", generators["go"],
		[]string{"module:", "  path: " + goModule, `  version: "1.22"`, "packageName: " + lower})[1:]...)
	lines = append(lines, group("typescript", "TypeScript, as sources (.ts).", generators["typescript"], typescript)...)
	lines = append(lines, group("typescript-dist", "TypeScript, compiled (.js and .d.ts): what a Worker or a Node program imports.", generators["typescript"],
		append(slices.Clone(typescript), "outputSourceFiles: false"))...)
	if slices.Contains(sdks.More, "python") {
		lines = append(lines, group("python", "Python.", generators["python"], nil)...)
	}
	if slices.Contains(sdks.More, "cli") {
		lines = append(lines, group("cli", "A command-line client, in Rust (Fern's early access).", generators["cli"], []string{"binaryName: " + lower})...)
	}
	for _, audience := range sdks.Audiences {
		lines = append(lines, group("typescript-"+audience, "TypeScript, only what the contract tags for the audience "+audience+".", generators["typescript"],
			typescript, "    audiences:", "      - "+audience)...)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

type keyValue struct {
	key   string
	value json.RawMessage
}

// ordered is a JSON object's members in its own order (a Go map has none); none for anything else.
func ordered(raw json.RawMessage) []keyValue {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil
	}
	var out []keyValue
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return out
		}
		var v json.RawMessage
		if dec.Decode(&v) != nil {
			return out
		}
		out = append(out, keyValue{t.(string), v})
	}
	return out
}
