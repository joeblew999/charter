package auth

import (
	"net/http"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/lib/problem"

	"github.com/joeblew999/charter/go/lib/internal/reply"
)

// How a client finds out how to get in: what an MCP client reads on a 401 (MCP's authorization, on
// OAuth 2.0 Protected Resource Metadata, RFC 9728), and what any OAuth client reads of an API that
// issues its own tokens (Authorization Server Metadata, RFC 8414).
const (
	// resourcePath is the API's protected-resource metadata: the authorization servers it trusts, and
	// its scopes. A resource with a path has it after this one (RFC 9728, 3.1).
	resourcePath = "/.well-known/oauth-protected-resource"
	// serverPath is the authorization-server metadata of an API that issues tokens itself (Issued
	// with a TokenPath): its token endpoint, and how a client authenticates there.
	serverPath = "/.well-known/oauth-authorization-server"
	// openIDPath is the specs' openIdConnectUrl: on to the trusted issuer's discovery document.
	openIDPath = "/.well-known/openid-configuration"
)

// WellKnown serves what a client reads to find out how to get in, for api with the ways in trusted:
// resourcePath (and a path after it), serverPath when the API issues tokens itself, and openIDPath.
// Anything else is 404 (problem details). Mount it at /.well-known/.
func WellKnown(api huma.API, setting func(name string) string, trusted ...Trusted) http.Handler {
	if setting == nil {
		setting = func(string) string { return "" }
	}
	discovery := discoveryHandler(setting)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := originOf(r.URL.Scheme, r.URL.Host, r.Host, r.TLS != nil, r.Header.Get("X-Forwarded-Proto"))
		switch path := r.URL.Path; {
		case path == openIDPath:
			discovery.ServeHTTP(w, r)
		case path == resourcePath || strings.HasPrefix(path, resourcePath+"/"):
			metadata := map[string]any{
				"resource":                 origin + strings.TrimPrefix(path, resourcePath),
				"authorization_servers":    authorizationServers(origin, setting, trusted),
				"scopes_supported":         apiScopes(api.OpenAPI()),
				"bearer_methods_supported": []string{"header"},
			}
			if info := api.OpenAPI().Info; info != nil && info.Title != "" {
				metadata["resource_name"] = info.Title
			}
			writeJSON(w, metadata)
		case path == serverPath && tokenPath(trusted) != "":
			writeJSON(w, map[string]any{
				"issuer":                                origin,
				"token_endpoint":                        origin + tokenPath(trusted),
				"grant_types_supported":                 []string{"client_credentials"},
				"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
				"response_types_supported":              []string{},
				"scopes_supported":                      apiScopes(api.OpenAPI()),
			})
		default:
			problem.Write(w, http.StatusNotFound, "not found")
		}
	})
}

// authorizationServers are the issuers of the tokens the API trusts, each set, in the order a client
// should try them (an MCP client takes the first): the API itself when it issues tokens (Issued with
// a TokenPath), an OpenID Connect issuer, the Cloudflare Access team.
func authorizationServers(origin string, setting func(string) string, trusted []Trusted) []string {
	servers := []string{}
	if tokenPath(trusted) != "" {
		servers = append(servers, origin)
	}
	if issuer := setting(OIDCIssuer); issuer != "" && slices.ContainsFunc(trusted, func(t Trusted) bool { _, ok := t.(OIDC); return ok }) {
		servers = append(servers, strings.TrimSuffix(issuer, "/"))
	}
	if team := setting(AccessTeam); team != "" && slices.ContainsFunc(trusted, func(t Trusted) bool { _, ok := t.(Access); return ok }) {
		if !strings.Contains(team, "://") {
			team = "https://" + team
		}
		servers = append(servers, strings.TrimSuffix(team, "/"))
	}
	return slices.Compact(servers)
}

// tokenPath is the token endpoint of the API's own tokens, if it issues any.
func tokenPath(trusted []Trusted) string {
	for _, t := range trusted {
		if issued, ok := t.(Issued); ok && issued.TokenPath != "" {
			return issued.TokenPath
		}
	}
	return ""
}

// apiScopes is every scope the API's security names, sorted.
func apiScopes(spec *huma.OpenAPI) []string {
	scopes := []string{}
	add := func(security []map[string][]string) {
		for _, requirement := range security {
			for _, scope := range scopesIn(requirement) {
				if !slices.Contains(scopes, scope) {
					scopes = append(scopes, scope)
				}
			}
		}
	}
	add(spec.Security)
	if spec.Paths != nil {
		for _, item := range spec.Paths {
			for _, op := range []*huma.Operation{item.Get, item.Put, item.Post, item.Delete, item.Options, item.Head, item.Patch, item.Trace} {
				if op != nil {
					add(op.Security)
				}
			}
		}
	}
	slices.Sort(scopes)
	return scopes
}

// challenge is the WWW-Authenticate a refusal carries: Bearer, the error if there is one
// (invalid_token, insufficient_scope with the scopes it needs), and where the API says how to get in.
func challenge(origin, error string, scopes []string) string {
	parts := []string{}
	if error != "" {
		parts = append(parts, `error="`+error+`"`)
	}
	if len(scopes) > 0 {
		parts = append(parts, `scope="`+strings.Join(scopes, " ")+`"`)
	}
	parts = append(parts, `resource_metadata="`+origin+resourcePath+`"`)
	return "Bearer " + strings.Join(parts, ", ")
}

// originOf is a request's scheme and host: absolute in its URL on Workers, from Host natively, https
// when it came over TLS or a proxy says so.
func originOf(scheme, urlHost, host string, tls bool, forwarded string) string {
	if scheme == "" {
		scheme = "http"
		if tls {
			scheme = "https"
		}
		if forwarded != "" {
			scheme = forwarded
		}
	}
	if urlHost != "" {
		host = urlHost
	}
	return scheme + "://" + host
}

// Origin is a request's scheme and host: absolute in its URL on Workers, from Host natively, https
// when it came over TLS or a proxy says so.
func Origin(r *http.Request) string {
	return originOf(r.URL.Scheme, r.URL.Host, r.Host, r.TLS != nil, r.Header.Get("X-Forwarded-Proto"))
}

// contextOrigin is originOf for a Huma request.
func contextOrigin(ctx huma.Context) string {
	u := ctx.URL()
	return originOf(u.Scheme, u.Host, ctx.Host(), ctx.TLS() != nil, ctx.Header("X-Forwarded-Proto"))
}

// writeJSON answers with a document any page may read (CORS: what a browser client asks for).
func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	reply.JSON(w, http.StatusOK, body)
}
