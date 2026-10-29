// Package standard is the standard test: what every API charter serves must do (docs/self/standard.md), checked on a
// running API from outside, as a client sees it. It knows no API: it reads the one it tests (its /api/openapi.json)
// and checks it against that. charter check runs it on the project's server (natively and under workerd), charter
// live-test on the deployed Worker, so no project writes or copies it.
//   - both specs are served, and name the API's own address;
//   - a refusal is problem details: an unknown path is 404, a method a path does not have is 405 with the methods
//     it has;
//   - its MCP endpoint lists tools that are the spec's operations, and answers MCP's protocol errors;
//   - an operation that needs credentials, called without them, is 401 with the challenge, which leads to the API's
//     metadata, which names the API and the scopes; through MCP too.
//
// Behind Cloudflare Access, every request carries the service token in <WORKER>_ACCESS_CLIENT_ID and _SECRET
// (WORKER: the first label of the host, in capitals), as the live tests send it.
package standard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Methods are the methods an OpenAPI path item may have.
var Methods = []string{"get", "put", "post", "delete", "patch"}

type operation struct {
	OperationID string                 `json:"operationId"`
	Security    *[]map[string][]string `json:"security"`
	RequestBody *struct {
		Content map[string]json.RawMessage `json:"content"`
	} `json:"requestBody"`
}

type spec struct {
	Servers  []struct{ URL string }                `json:"servers"`
	Paths    map[string]map[string]json.RawMessage `json:"paths"`
	Security []map[string][]string                 `json:"security"`
}

type op struct {
	path, method string
	op           operation
}

// Run runs the standard test on the API at origin, says each check, and fails if one did.
func Run(origin string, say func(string)) error {
	access := accessHeaders(origin)
	var failures []string
	check := func(name string, ok bool, detail any) {
		if ok {
			say("PASS  standard: " + name)
			return
		}
		d, isString := detail.(string)
		if !isString {
			b, _ := json.Marshal(detail)
			d = string(b)
		}
		say("FAIL  standard: " + name + ": " + d)
		failures = append(failures, name)
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	type answer struct {
		status int
		header http.Header
		body   []byte
	}
	do := func(method, path string, header map[string]string, body string) answer {
		req, err := http.NewRequest(method, origin+path, strings.NewReader(body))
		if err != nil {
			return answer{}
		}
		for k, v := range access {
			req.Header.Set(k, v)
		}
		for k, v := range header {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return answer{header: http.Header{}}
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return answer{resp.StatusCode, resp.Header, b}
	}
	get := func(path string) answer { return do("GET", path, nil, "") }
	object := func(a answer) map[string]any {
		var m map[string]any
		json.Unmarshal(a.body, &m)
		return m
	}
	problem := func(a answer, status int) bool {
		m := object(a)
		s, _ := m["status"].(float64)
		return a.status == status && strings.HasPrefix(a.header.Get("content-type"), "application/problem+json") && int(s) == status
	}
	mcp := func(method string, params map[string]any, raw string) answer {
		header, body := Request(method, params, "charter-standard", raw)
		return do("POST", "/api/mcp", header, body)
	}

	// Both specs, naming this address.
	specAnswer := get("/api/openapi.json")
	var s spec
	json.Unmarshal(specAnswer.body, &s)
	servers := ""
	if len(s.Servers) > 0 {
		servers = s.Servers[0].URL
	}
	check("GET /api/openapi.json names this address", specAnswer.status == 200 && servers == origin, map[string]any{"status": specAnswer.status, "servers": s.Servers})
	asyncAnswer := get("/api/asyncapi.json")
	var async struct {
		Channels map[string]any `json:"channels"`
		Servers  struct {
			Production struct{ Host string } `json:"production"`
		} `json:"servers"`
	}
	json.Unmarshal(asyncAnswer.body, &async)
	host := ""
	if u, err := url.Parse(origin); err == nil {
		host = u.Host
	}
	check("GET /api/asyncapi.json, naming this host if it has channels", asyncAnswer.status == 200 && (len(async.Channels) == 0 || async.Servers.Production.Host == host),
		map[string]any{"status": asyncAnswer.status, "servers": async.Servers})

	// Refusals are problem details.
	check("an unknown path is 404, as problem details", problem(get("/api/charter-standard-unknown"), 404), "not a 404 problem")
	// The operations in the spec's own order, as a client reading it meets them.
	var ops []op
	paths := orderedPaths(specAnswer.body)
	for _, p := range paths {
		for _, m := range Methods {
			if raw, ok := s.Paths[p][m]; ok {
				var o operation
				json.Unmarshal(raw, &o)
				ops = append(ops, op{p, m, o})
			}
		}
	}
	// The first path without parameters.
	if i := slices.IndexFunc(paths, func(p string) bool { return !strings.Contains(p, "{") }); i >= 0 {
		p := paths[i]
		var has []string
		for _, m := range Methods {
			if _, ok := s.Paths[p][m]; ok {
				has = append(has, strings.ToUpper(m))
			}
		}
		slices.Sort(has)
		other := ""
		for _, m := range []string{"DELETE", "PUT", "PATCH", "POST", "GET"} {
			if !slices.Contains(has, m) {
				other = m
				break
			}
		}
		a := do(other, p, nil, "")
		check(fmt.Sprintf("%s %s is 405, with the methods it has", other, p), problem(a, 405) && a.header.Get("allow") == strings.Join(has, ", "),
			map[string]any{"status": a.status, "allow": a.header.Get("allow")})
	}

	// MCP: the tools are the spec's operations, and MCP's own errors.
	listed := mcp("tools/list", map[string]any{}, "")
	var list struct {
		Result struct {
			Tools []struct{ Name string } `json:"tools"`
		} `json:"result"`
	}
	json.Unmarshal(listed.body, &list)
	ids := map[string]bool{}
	for _, o := range ops {
		ids[o.op.OperationID] = true
	}
	var tools []string
	every := true
	for _, t := range list.Result.Tools {
		tools = append(tools, t.Name)
		every = every && ids[t.Name]
	}
	check("POST /api/mcp lists tools, each an operation of the spec", listed.status == 200 && every, map[string]any{"status": listed.status, "tools": tools})
	getMcp := get("/api/mcp")
	check("GET /api/mcp is 405, with Allow: POST", getMcp.status == 405 && getMcp.header.Get("allow") == "POST", map[string]any{"status": getMcp.status, "allow": getMcp.header.Get("allow")})
	code := func(a answer) float64 {
		e, _ := object(a)["error"].(map[string]any)
		c, _ := e["code"].(float64)
		return c
	}
	parse := mcp("tools/list", map[string]any{}, `{"jsonrpc":"2.0","id":1,"method":`)
	check("MCP: JSON that does not parse is error -32700", code(parse) == -32700, string(parse.body))
	unknown := mcp("charter/standard", map[string]any{}, "")
	check("MCP: an unknown method is error -32601", code(unknown) == -32601, string(unknown.body))

	// Signing in: an operation that needs credentials, called without them.
	security := func(o operation) []map[string][]string {
		if o.Security != nil {
			return *o.Security
		}
		return s.Security
	}
	needs := func(o operation) bool {
		return slices.ContainsFunc(security(o), func(r map[string][]string) bool { return len(r) > 0 })
	}
	i := slices.IndexFunc(ops, func(o op) bool { return needs(o.op) })
	if i < 0 {
		check("an API that takes no credentials says nothing at /.well-known/", get("/.well-known/oauth-protected-resource").status == 404, "not 404")
	} else {
		secured := ops[i]
		path := regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(secured.path, "x")
		header, body := map[string]string{}, ""
		if secured.op.RequestBody != nil && secured.op.RequestBody.Content["application/json"] != nil {
			header["content-type"], body = "application/json", "{}"
		}
		refused := do(strings.ToUpper(secured.method), path, header, body)
		challenge := refused.header.Get("www-authenticate")
		metadataURL := origin + "/.well-known/oauth-protected-resource"
		check(fmt.Sprintf("%s %s without credentials is 401, whose challenge points to the API's metadata", strings.ToUpper(secured.method), secured.path),
			problem(refused, 401) && strings.Contains(challenge, `resource_metadata="`+metadataURL+`"`), map[string]any{"status": refused.status, "challenge": challenge})
		var metadata struct {
			Resource             string   `json:"resource"`
			ScopesSupported      []string `json:"scopes_supported"`
			AuthorizationServers []string `json:"authorization_servers"`
		}
		metadataAnswer := get("/.well-known/oauth-protected-resource")
		json.Unmarshal(metadataAnswer.body, &metadata)
		scopesOK := true
		for _, r := range security(secured.op) {
			for _, scopes := range r {
				for _, scope := range scopes {
					scopesOK = scopesOK && slices.Contains(metadata.ScopesSupported, scope)
				}
			}
		}
		check("the API's metadata names it and its scopes", metadata.Resource == origin && scopesOK, string(metadataAnswer.body))
		if slices.Contains(metadata.AuthorizationServers, origin) {
			var server struct {
				TokenEndpoint string `json:"token_endpoint"`
			}
			serverAnswer := get("/.well-known/oauth-authorization-server")
			json.Unmarshal(serverAnswer.body, &server)
			tokenPath, ok := strings.CutPrefix(server.TokenEndpoint, origin)
			_, post := s.Paths[tokenPath]["post"]
			check("the API, its own authorization server, names its token endpoint, a route of the spec", ok && tokenPath != "" && post, string(serverAnswer.body))
		}
		toolNames := map[string]bool{}
		for _, t := range tools {
			toolNames[t] = true
		}
		if j := slices.IndexFunc(ops, func(o op) bool { return o.op.OperationID != "" && toolNames[o.op.OperationID] && needs(o.op) }); j >= 0 {
			name := ops[j].op.OperationID
			call := mcp("tools/call", map[string]any{"name": name, "arguments": map[string]any{}}, "")
			c := call.header.Get("www-authenticate")
			check(fmt.Sprintf("MCP: %s without credentials is 401, with the challenge", name), call.status == 401 && strings.Contains(c, `resource_metadata="`+metadataURL+`"`),
				map[string]any{"status": call.status, "challenge": c})
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("the standard test failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

// orderedPaths are the spec's paths in its own order (a Go map has none).
func orderedPaths(specJSON []byte) []string {
	var raw struct {
		Paths json.RawMessage `json:"paths"`
	}
	if json.Unmarshal(specJSON, &raw) != nil {
		return nil
	}
	return Keys(raw.Paths)
}

// Keys are a JSON object's keys in its own order (a Go map has none); none when it is no object.
func Keys(object json.RawMessage) []string {
	if len(object) == 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(object)))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil
	}
	var out []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return out
		}
		key, _ := t.(string)
		var skip json.RawMessage
		if dec.Decode(&skip) != nil {
			return out
		}
		out = append(out, key)
	}
	return out
}

// Request is an MCP request to /api/mcp as a client sends it (the stateless protocol), from client: its headers
// and body; raw is a body sent as it is, in place of the request.
func Request(method string, params map[string]any, client, raw string) (map[string]string, string) {
	const version = "2026-07-28"
	header := map[string]string{"content-type": "application/json", "accept": "application/json, text/event-stream", "mcp-protocol-version": version, "mcp-method": method}
	if name, ok := params["name"].(string); ok && name != "" {
		header["mcp-name"] = name
	}
	if raw != "" {
		return header, raw
	}
	p := map[string]any{"_meta": map[string]any{
		"io.modelcontextprotocol/protocolVersion":    version,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": client, "version": "1"},
	}}
	for k, v := range params {
		p[k] = v
	}
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": p})
	return header, string(b)
}

// accessHeaders is the Cloudflare Access service token for a host, from the environment; none when it is not set.
func accessHeaders(origin string) map[string]string {
	u, err := url.Parse(origin)
	if err != nil {
		return nil
	}
	label, _, _ := strings.Cut(u.Hostname(), ".")
	prefix := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(label))
	id, secret := os.Getenv(prefix+"_ACCESS_CLIENT_ID"), os.Getenv(prefix+"_ACCESS_CLIENT_SECRET")
	if id == "" || secret == "" {
		return nil
	}
	return map[string]string{"cf-access-client-id": id, "cf-access-client-secret": secret}
}
