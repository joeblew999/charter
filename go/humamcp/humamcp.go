// Package humamcp serves a Huma API as an MCP server (Model Context Protocol), the way Huma itself
// serves it as OpenAPI: every operation is a tool, its input struct is the tool's inputSchema, and a
// tool call runs the operation's own HTTP handler, so MCP gets the same validation, the same errors
// and the same code as REST.
//
// It is MCP's Streamable HTTP transport written out by hand: JSON-RPC 2.0, one POST per message,
// one application/json answer. No MCP SDK: the Go ones don't build with TinyGo, and nothing here
// needs one. It keeps no state, which is what workers-go needs (a fresh Go runtime per request) and
// what MCP became in revision 2026-07-28 (no handshake, no sessions). Both eras are served on the
// one endpoint:
//
//   - 2026-07-28: every request names its version in params._meta; server/discover, tools/list,
//     tools/call.
//   - 2025-11-25 and 2025-06-18: initialize (no Mcp-Session-Id is given, so none is needed),
//     notifications/initialized, ping, tools/list, tools/call.
//
// Only tools, and no stream: GET is 405, there are no resources, prompts, progress notifications or
// list-changed subscriptions, and no authorization of its own (the Authorization header is passed on
// to the operation). tools.go says how an operation becomes a tool.
//
// The rules followed are those of https://modelcontextprotocol.io/specification/2026-07-28
// (basic, basic/versioning, basic/transports/streamable-http, server/discover, server/tools) and,
// for the handshake, https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle.
package humamcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/orpc-api/go/humaworkers"
)

// The protocol revisions served: Modern ones say their version on every request, Legacy ones open
// with initialize.
var (
	Modern = []string{"2026-07-28"}
	Legacy = []string{"2025-11-25", "2025-06-18"}
)

// JSON-RPC and MCP error codes.
const (
	parseError         = -32700
	invalidRequest     = -32600
	methodNotFound     = -32601
	invalidParams      = -32602
	headerMismatch     = -32020
	unsupportedVersion = -32022
)

const (
	metaVersion      = "io.modelcontextprotocol/protocolVersion"
	metaCapabilities = "io.modelcontextprotocol/clientCapabilities"
	metaServer       = "io.modelcontextprotocol/serverInfo"

	// maxBody is Huma's own default limit for a request body.
	maxBody = 1 << 20
	// listTTL is how long a client may keep tools/list and server/discover: they only change with a deploy.
	listTTL = 300_000
)

// Handler serves api's operations as MCP tools. Mount it on one path (e.g. /api/mcp), next to the
// API. It is cheap to make: nothing is registered until a request needs it (tools/list needs every
// operation, tools/call only the one called).
func Handler(api *humaworkers.API) http.Handler { return server{api} }

type server struct{ api *humaworkers.API }

// message is a JSON-RPC request or notification.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// params are the parts of params that are read here.
type params struct {
	Meta            map[string]json.RawMessage `json:"_meta"`
	ProtocolVersion string                     `json:"protocolVersion"` // initialize
	Name            string                     `json:"name"`            // tools/call
	Arguments       json.RawMessage            `json:"arguments"`       // tools/call
}

func (s server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A browser page on another site must not reach the server (the spec's Origin rule).
	if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r) {
		fail(w, http.StatusForbidden, nil, invalidRequest, "Origin "+origin+" is not this server's", nil)
		return
	}
	// No stream to open (GET) and no session to end (DELETE).
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed: MCP messages are POSTed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		fail(w, http.StatusRequestEntityTooLarge, nil, invalidRequest, "the message could not be read, or is over 1 MB", nil)
		return
	}
	// One message per POST: batches left the protocol in 2025-06-18.
	if body = bytes.TrimSpace(body); len(body) > 0 && body[0] == '[' {
		fail(w, http.StatusBadRequest, nil, invalidRequest, "Invalid Request: batches are not supported", nil)
		return
	}
	var msg message
	if err := json.Unmarshal(body, &msg); err != nil {
		fail(w, http.StatusBadRequest, nil, parseError, "Parse error: "+err.Error(), nil)
		return
	}
	if msg.JSONRPC != "2.0" || msg.Method == "" || string(msg.ID) == "null" {
		fail(w, http.StatusBadRequest, nil, invalidRequest, "Invalid Request: expected a JSON-RPC 2.0 request or notification", nil)
		return
	}
	// A notification (notifications/initialized, notifications/cancelled): accepted, nothing to do.
	if len(msg.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var p params
	if len(msg.Params) > 0 {
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			fail(w, http.StatusOK, msg.ID, invalidParams, "Invalid params: "+err.Error(), nil)
			return
		}
	}

	var version string
	modern := false
	if raw, ok := p.Meta[metaVersion]; ok {
		modern = json.Unmarshal(raw, &version) == nil
	}
	header := r.Header.Get("MCP-Protocol-Version")
	supported := map[string]any{"supported": append(append([]string{}, Modern...), Legacy...)}
	if !modern {
		// The handshake era. Its requests may name the version negotiated by initialize.
		if contains(Modern, header) {
			fail(w, http.StatusBadRequest, msg.ID, invalidParams, "Invalid params: params._meta needs "+metaVersion+" and "+metaCapabilities, nil)
			return
		}
		if header != "" && !contains(Legacy, header) {
			supported["requested"] = header
			fail(w, http.StatusBadRequest, msg.ID, unsupportedVersion, "Unsupported protocol version", supported)
			return
		}
		s.legacy(w, r, msg, p)
		return
	}

	// The stateless era: the version and the routing headers must say what the body says.
	if header != version {
		fail(w, http.StatusBadRequest, msg.ID, headerMismatch, "Header mismatch: MCP-Protocol-Version header value '"+header+"' does not match body value '"+version+"'", nil)
		return
	}
	if !contains(Modern, version) {
		supported["requested"] = version
		fail(w, http.StatusBadRequest, msg.ID, unsupportedVersion, "Unsupported protocol version", supported)
		return
	}
	if got := r.Header.Get("Mcp-Method"); got != msg.Method {
		fail(w, http.StatusBadRequest, msg.ID, headerMismatch, "Header mismatch: Mcp-Method header value '"+got+"' does not match body value '"+msg.Method+"'", nil)
		return
	}
	if got := decoded(r.Header.Get("Mcp-Name")); msg.Method == "tools/call" && got != p.Name {
		fail(w, http.StatusBadRequest, msg.ID, headerMismatch, "Header mismatch: Mcp-Name header value '"+got+"' does not match body value '"+p.Name+"'", nil)
		return
	}
	if _, ok := p.Meta[metaCapabilities]; !ok {
		fail(w, http.StatusBadRequest, msg.ID, invalidParams, "Invalid params: params._meta needs "+metaCapabilities, nil)
		return
	}
	var result map[string]any
	switch msg.Method {
	case "server/discover":
		result = map[string]any{"supportedVersions": supported["supported"], "capabilities": capabilities, "ttlMs": listTTL, "cacheScope": "public"}
		if instructions := s.info().Description; instructions != "" {
			result["instructions"] = instructions
		}
	case "tools/list":
		result = s.list()
		result["ttlMs"], result["cacheScope"] = listTTL, "public"
	case "tools/call":
		if result = s.call(w, r, msg, p); result == nil {
			return
		}
	default:
		fail(w, http.StatusNotFound, msg.ID, methodNotFound, "Method not found: "+msg.Method, nil)
		return
	}
	// Every result says it is final (there are no input_required ones) and who answered.
	result["resultType"] = "complete"
	result["_meta"] = map[string]any{metaServer: s.implementation()}
	reply(w, msg.ID, result)
}

// legacy answers a request of the handshake era. Nothing is remembered between requests, so
// initialize only reports, and every other request is answered whether or not it came first.
func (s server) legacy(w http.ResponseWriter, r *http.Request, msg message, p params) {
	var result map[string]any
	switch msg.Method {
	case "initialize":
		// The client's version if it is served, otherwise the newest one of this era: the client decides.
		version := p.ProtocolVersion
		if !contains(Legacy, version) {
			version = Legacy[0]
		}
		result = map[string]any{"protocolVersion": version, "capabilities": capabilities, "serverInfo": s.implementation()}
		if instructions := s.info().Description; instructions != "" {
			result["instructions"] = instructions
		}
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result = s.list()
	case "tools/call":
		result = s.call(w, r, msg, p)
	default:
		fail(w, http.StatusOK, msg.ID, methodNotFound, "Method not found: "+msg.Method, nil)
		return
	}
	if result != nil {
		reply(w, msg.ID, result)
	}
}

// capabilities: tools, whose list never changes while the server runs.
var capabilities = map[string]any{"tools": map[string]any{}}

func (s server) info() *huma.Info {
	// The embedded huma.API's document: asking for it registers nothing.
	if info := s.api.API.OpenAPI().Info; info != nil {
		return info
	}
	return &huma.Info{}
}

// implementation is serverInfo: the API's title and version.
func (s server) implementation() map[string]any {
	return map[string]any{"name": s.info().Title, "version": s.info().Version}
}

// list is the result of tools/list.
func (s server) list() map[string]any {
	return map[string]any{"tools": Tools(s.api.Operations(), s.api.API.OpenAPI().Components.Schemas)}
}

// call is the result of tools/call, or nil when it has answered with an error. Only a call that
// can't be made is a protocol error; whatever the operation answers, 4xx and 5xx too, is a result.
func (s server) call(w http.ResponseWriter, r *http.Request, msg message, p params) map[string]any {
	op := s.api.Operation(p.Name)
	if p.Name == "" || op == nil || !IsTool(op) {
		fail(w, http.StatusOK, msg.ID, invalidParams, "Unknown tool: "+p.Name, nil)
		return nil
	}
	arguments := map[string]any{}
	if len(p.Arguments) > 0 && string(p.Arguments) != "null" {
		// Numbers stay as written: a 64-bit id must not pass through a float.
		decoder := json.NewDecoder(bytes.NewReader(p.Arguments))
		decoder.UseNumber()
		if err := decoder.Decode(&arguments); err != nil {
			fail(w, http.StatusOK, msg.ID, invalidParams, "Invalid params: arguments must be an object", nil)
			return nil
		}
	}
	return run(s.api, r, op, arguments)
}

func reply(w http.ResponseWriter, id json.RawMessage, result any) {
	write(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

// fail answers with a JSON-RPC error. id is nil when the request's id could not be read.
func fail(w http.ResponseWriter, status int, id json.RawMessage, code int, text string, data any) {
	failure := map[string]any{"code": code, "message": text}
	if data != nil {
		failure["data"] = data
	}
	body := map[string]any{"jsonrpc": "2.0", "error": failure}
	if id != nil {
		body["id"] = id
	}
	write(w, status, body)
}

func write(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data)
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// decoded reads an Mcp-Name header: plain, or =?base64?...?= when the name is not header-safe.
func decoded(value string) string {
	if encoded, ok := strings.CutPrefix(value, "=?base64?"); ok && strings.HasSuffix(encoded, "?=") {
		if plain, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(encoded, "?=")); err == nil {
			return string(plain)
		}
	}
	return value
}

// sameOrigin reports whether an Origin header names the host the request came to.
func sameOrigin(origin string, r *http.Request) bool {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && u.Host == host
}
