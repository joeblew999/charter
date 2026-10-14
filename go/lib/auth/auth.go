// Package auth is who may call a Huma API, and with what scopes: the contract declares what each
// operation needs, and one middleware enforces it, so the specs, the SDKs and the server agree.
//
// A caller proves who it is in one of three ways, and the API lists those it trusts:
//
//   - [Token]: a bearer token, one of the Worker's secrets (READ_TOKEN, WRITE_TOKEN), with the
//     scopes it grants.
//   - [Access]: Cloudflare Access in front of the Worker. A person logs in (with GitHub, say), a
//     machine shows its own service token; Access sends the Worker a JWT in Cf-Access-Jwt-Assertion,
//     verified here against the team's keys and the application's AUD tag (the settings
//     ACCESS_TEAM_DOMAIN and ACCESS_AUD). People and machines get the scopes the API gives them.
//   - [OIDC]: an access token from an OpenID Connect issuer, in Authorization: Bearer, verified
//     here against the issuer's keys, found by its discovery document (the settings OIDC_ISSUER and
//     OIDC_AUDIENCE). The token's own scope claim is what it may do.
//
// The schemes go into the specs with [Scheme], [AccessScheme] and [OIDCScheme]; an operation needs
// the scopes its Security lists ([Needs]; the API's default when it lists none; nothing when its
// Security is [Public]), by any of the schemes. No credentials, or credentials that do not verify:
// 401. Known, without a scope the operation needs: 403. An unset setting trusts nothing, so a
// Worker without its secrets refuses everything that is not public. The handler finds the caller
// with [CallerOf].
//
// JWTs are verified with go-jose (github.com/go-jose/go-jose/v4): RS256, ES256 and EdDSA only, the
// issuer, the audience, exp (required) and nbf with a minute's leeway. Keys are fetched once and
// kept for an hour; a token whose key is not among them makes one new fetch, at most every 30 s.
package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// The security schemes' names in the specs.
const (
	// Name is the bearer tokens' scheme (Token).
	Name = "bearer"
	// AccessClientID and AccessClientSecret are a Cloudflare Access service token: the two headers
	// a machine sends, which Access checks at the edge before it lets the request through.
	AccessClientID     = "accessClientId"
	AccessClientSecret = "accessClientSecret"
	// OIDCName is an OpenID Connect issuer's access token.
	OIDCName = "oidc"
)

// The settings (the Worker's secrets or variables) that say whom the API trusts. Unset, it trusts
// no one that way.
const (
	AccessTeam   = "ACCESS_TEAM_DOMAIN" // the Access team: <team>.cloudflareaccess.com
	AccessAUD    = "ACCESS_AUD"         // the Access application's AUD tag
	OIDCIssuer   = "OIDC_ISSUER"        // the issuer: its discovery document is at <issuer>/.well-known/openid-configuration
	OIDCAudience = "OIDC_AUDIENCE"      // what the aud claim of a token for this API holds
)

// AccessHeader is the header Cloudflare Access puts its JWT in, on every request it lets through.
const AccessHeader = "Cf-Access-Jwt-Assertion"

// Trusted is a way in that Middleware accepts: a Token, Access or OIDC.
type Trusted interface{ trusted() }

// Token is one secret and the scopes it grants.
type Token struct {
	Secret string   // the Worker's secret (the environment variable) that holds the token
	Scopes []string // what it may do, e.g. read; write
}

// Access trusts Cloudflare Access for the application in ACCESS_TEAM_DOMAIN and ACCESS_AUD: a
// person who logged in gets People, a machine with a service token Machines. Which people and which
// tokens Access lets through is Access's policy (charter access setup).
type Access struct {
	People   []string
	Machines []string
}

// OIDC trusts the OpenID Connect issuer in OIDC_ISSUER, for tokens whose audience is OIDC_AUDIENCE.
// A token grants the scopes in its scope claim (or scp).
type OIDC struct{}

func (Token) trusted()  {}
func (Access) trusted() {}
func (OIDC) trusted()   {}

// Caller is who a request comes from, once its credentials are verified, and what it may do.
type Caller struct {
	Scheme  string   // what vouched for it: Name (a token), "access" or OIDCName
	Email   string   // a person: from Access, or from an OIDC token that has it
	Machine string   // an Access service token's Client ID (the JWT's common_name)
	Subject string   // the token's sub: a user's id at the issuer
	Token   string   // a bearer token: the secret that holds it, e.g. WRITE_TOKEN
	Scopes  []string // what it may do
}

// Name is the caller as a log line or an error message would name it.
func (c Caller) Name() string {
	switch {
	case c.Email != "":
		return c.Email
	case c.Machine != "":
		return "service token " + c.Machine
	case c.Token != "":
		return "the token " + c.Token
	}
	return c.Subject
}

type callerKey struct{}

// CallerOf is the caller the middleware let in, for an operation that needs scopes.
func CallerOf(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(callerKey{}).(Caller)
	return c, ok
}

// Scheme declares bearer tokens in the specs (config.OpenAPI), and the scopes an operation needs
// when it says nothing (none given: such operations are public). An operation that needs other
// scopes says so: Security: auth.Needs("write").
func Scheme(spec *huma.OpenAPI, scopes ...string) {
	declare(spec, Name, &huma.SecurityScheme{Type: "http", Scheme: "bearer"})
	if len(scopes) > 0 {
		spec.Security = expand(spec, Needs(scopes...))
	}
}

// AccessScheme declares a Cloudflare Access service token in the specs: the two headers, which
// the SDKs take as options or read from <API>_ACCESS_CLIENT_ID and <API>_ACCESS_CLIENT_SECRET, where
// <API> is api (the API's name, e.g. its Worker's) in capitals with _ for -.
func AccessScheme(spec *huma.OpenAPI, api string) {
	prefix := EnvPrefix(api)
	header := func(header, option, variable, what string) *huma.SecurityScheme {
		return &huma.SecurityScheme{
			Type: "apiKey", In: "header", Name: header, Description: what,
			Extensions: map[string]any{"x-fern-header": map[string]any{"name": option, "env": prefix + variable}},
		}
	}
	declare(spec, AccessClientID, header("CF-Access-Client-Id", "accessClientId", "_ACCESS_CLIENT_ID", "A Cloudflare Access service token's Client ID: a machine's own (charter access token create)"))
	declare(spec, AccessClientSecret, header("CF-Access-Client-Secret", "accessClientSecret", "_ACCESS_CLIENT_SECRET", "The service token's Client Secret"))
}

// OIDCScheme declares an OpenID Connect issuer's access tokens in the specs. Its discovery URL is
// the API's /.well-known/openid-configuration, which Discovery serves.
func OIDCScheme(spec *huma.OpenAPI) {
	declare(spec, OIDCName, &huma.SecurityScheme{
		Type: "openIdConnect", OpenIDConnectURL: "/.well-known/openid-configuration",
		Description: "An access token from the OpenID Connect issuer the deployment trusts (OIDC_ISSUER), with the operation's scopes",
	})
}

// EnvPrefix is the start of the variables an API's SDKs read: its name in capitals, _ for -.
func EnvPrefix(api string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(api))
}

// declare adds a scheme, and has every operation's Security name each scheme the API declares.
func declare(spec *huma.OpenAPI, name string, scheme *huma.SecurityScheme) {
	if spec.Components == nil {
		spec.Components = &huma.Components{}
	}
	if spec.Components.SecuritySchemes == nil {
		spec.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	first := !slices.ContainsFunc(ours, func(n string) bool { return spec.Components.SecuritySchemes[n] != nil })
	spec.Components.SecuritySchemes[name] = scheme
	spec.Security = expand(spec, spec.Security)
	if first {
		spec.OnAddOperation = append(spec.OnAddOperation, func(spec *huma.OpenAPI, op *huma.Operation) {
			op.Security = expand(spec, op.Security)
			if op.Security == nil && len(spec.Security) == 0 {
				op.Security = Public() // public, said so: generated SDKs then send nothing it does not need
			}
		})
	}
}

// Needs is an operation's Security: all of these scopes, by any scheme the API declares.
func Needs(scopes ...string) []map[string][]string {
	return []map[string][]string{{Name: append([]string{}, scopes...)}}
}

// Public is an operation's Security when it needs no credentials.
func Public() []map[string][]string { return []map[string][]string{} }

// ours are the schemes this package declares, in the order an operation's Security lists them.
var ours = []string{Name, AccessClientID, AccessClientSecret, OIDCName}

// expand rewrites each requirement of security that names only this package's schemes as one
// requirement per scheme the spec declares, with the same scopes. The specs then say every way in,
// and the SDKs send whichever credentials they were given.
func expand(spec *huma.OpenAPI, security []map[string][]string) []map[string][]string {
	if len(security) == 0 || spec.Components == nil {
		return security
	}
	declared := spec.Components.SecuritySchemes
	var out []map[string][]string
	seen := map[string]bool{}
	add := func(requirement map[string][]string, key string) {
		if !seen[key] {
			seen[key] = true
			out = append(out, requirement)
		}
	}
	for _, requirement := range security {
		mine := true
		for name := range requirement {
			mine = mine && slices.Contains(ours, name)
		}
		if !mine {
			out = append(out, requirement)
			continue
		}
		scopes := scopesIn(requirement)
		key := strings.Join(scopes, " ")
		if declared[Name] != nil {
			add(map[string][]string{Name: scopes}, Name+" "+key)
		}
		if declared[AccessClientID] != nil {
			add(map[string][]string{AccessClientID: scopes, AccessClientSecret: {}}, AccessClientID+" "+key)
		}
		if declared[OIDCName] != nil {
			add(map[string][]string{OIDCName: scopes}, OIDCName+" "+key)
		}
	}
	if out == nil {
		return security
	}
	return out
}

// scopesIn is every scope a requirement names, whichever scheme names it.
func scopesIn(requirement map[string][]string) []string {
	scopes := []string{}
	names := make([]string, 0, len(requirement))
	for name := range requirement {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		for _, s := range requirement[name] {
			if !slices.Contains(scopes, s) {
				scopes = append(scopes, s)
			}
		}
	}
	return scopes
}

// Middleware enforces each operation's Security. setting looks a secret or variable up (the
// Worker's environment); trusted are the ways in the API accepts.
func Middleware(api huma.API, setting func(name string) string, trusted ...Trusted) func(huma.Context, func(huma.Context)) {
	// The API's document, for its default security: without registering routes when the API can
	// (humaworkers registers them as requests need them, after this middleware is in place).
	spec := api.OpenAPI
	if lazy, ok := api.(interface{ Spec() *huma.OpenAPI }); ok {
		spec = lazy.Spec
	}
	if setting == nil {
		setting = func(string) string { return "" }
	}
	g := gate{setting: setting, keys: &keyCache{}}
	for _, t := range trusted {
		switch t := t.(type) {
		case Token:
			g.tokens = append(g.tokens, t)
		case Access:
			g.access = &t
		case OIDC:
			g.oidc = &t
		}
	}
	return func(ctx huma.Context, next func(huma.Context)) {
		security := ctx.Operation().Security
		if security == nil {
			security = spec().Security
		}
		if len(security) == 0 {
			next(ctx)
			return
		}
		callers, err := g.identify(ctx.Context(), ctx.Header)
		if err != nil || len(callers) == 0 {
			ctx.SetHeader("WWW-Authenticate", "Bearer")
			message := "credentials are required: " + g.ways()
			if err != nil {
				message = "the credentials were refused: " + err.Error()
			}
			huma.WriteErr(api, ctx, http.StatusUnauthorized, message)
			return
		}
		// Security is a list of alternatives; the scopes of one are all needed.
		for _, alternative := range security {
			needs := scopesIn(alternative)
			for _, c := range callers {
				if !slices.ContainsFunc(needs, func(scope string) bool { return !slices.Contains(c.Scopes, scope) }) {
					next(huma.WithValue(ctx, callerKey{}, c))
					return
				}
			}
		}
		huma.WriteErr(api, ctx, http.StatusForbidden, callers[0].Name()+" may not do this: it needs "+describe(security))
	}
}

// gate is what the middleware trusts.
type gate struct {
	setting func(string) string
	tokens  []Token
	access  *Access
	oidc    *OIDC
	keys    *keyCache
}

// ways says what the API takes, for a 401.
func (g gate) ways() string {
	var ways []string
	if g.access != nil && g.setting(AccessTeam) != "" {
		ways = append(ways, "a Cloudflare Access login or service token (CF-Access-Client-Id, CF-Access-Client-Secret)")
	}
	if len(g.tokens) > 0 || g.oidc != nil {
		ways = append(ways, "Authorization: Bearer <token>")
	}
	return strings.Join(ways, ", or ")
}

// errUnknownToken is a bearer token that is none of the API's.
var errUnknownToken = errors.New("not a token this API knows")

// identify verifies every credential the request carries, and says who each one is. One that does
// not verify is an error; none at all is no callers.
func (g gate) identify(ctx context.Context, header func(string) string) ([]Caller, error) {
	var callers []Caller
	if jwt := header(AccessHeader); jwt != "" && g.access != nil {
		if team, aud := g.setting(AccessTeam), g.setting(AccessAUD); team != "" && aud != "" {
			c, err := g.keys.access(ctx, team, aud, jwt)
			if err != nil {
				return nil, err
			}
			c.Scopes = g.access.People
			if c.Machine != "" {
				c.Scopes = g.access.Machines
			}
			callers = append(callers, c)
		}
	}
	token := bearer(header("Authorization"))
	if token == "" {
		return callers, nil
	}
	if strings.Count(token, ".") == 2 && g.oidc != nil {
		if issuer, audience := g.setting(OIDCIssuer), g.setting(OIDCAudience); issuer != "" && audience != "" {
			c, err := g.keys.oidc(ctx, issuer, audience, token)
			if err != nil {
				return nil, err
			}
			return append(callers, c), nil
		}
	}
	if c, known := scopesOf(token, g.setting, g.tokens); known {
		return append(callers, c), nil
	}
	return nil, errUnknownToken
}

// scopesOf is the caller a bearer token is, and whether it is one of the tokens at all. It compares
// with every secret in constant time.
func scopesOf(token string, setting func(string) string, tokens []Token) (Caller, bool) {
	c := Caller{Scheme: Name}
	known := false
	for _, t := range tokens {
		want := setting(t.Secret)
		if want != "" && token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(want)) == 1 {
			if !known {
				c.Token = t.Secret
			}
			known = true
			c.Scopes = append(c.Scopes, t.Scopes...)
		}
	}
	return c, known
}

func bearer(authorization string) string {
	scheme, token, ok := strings.Cut(authorization, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func describe(security []map[string][]string) string {
	var alternatives []string
	for _, alternative := range security {
		if a := "scope " + strings.Join(scopesIn(alternative), " and "); !slices.Contains(alternatives, a) {
			alternatives = append(alternatives, a)
		}
	}
	return strings.Join(alternatives, ", or ")
}

// Discovery serves /.well-known/openid-configuration, the specs' openIdConnectUrl: a redirect to
// the discovery document of the issuer in OIDC_ISSUER, or 404 when there is none.
func Discovery(setting func(name string) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issuer := setting(OIDCIssuer)
		if issuer == "" {
			http.Error(w, "this API trusts no OpenID Connect issuer", http.StatusNotFound)
			return
		}
		http.Redirect(w, r, strings.TrimSuffix(issuer, "/")+"/.well-known/openid-configuration", http.StatusFound)
	})
}
