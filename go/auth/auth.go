// Package auth is bearer tokens with scopes for a Huma API: the contract declares what each
// operation needs, and one middleware enforces it, so the specs, the SDKs and the server agree.
//
// The scheme goes into the specs with [Scheme]; an operation needs the scopes its Security lists
// (the API's default when it lists none; nothing when its Security is an empty list). Each token is
// a secret of the Worker (READ_TOKEN, WRITE_TOKEN) with the scopes it grants. A request without a
// token it knows gets 401; a known token without a scope the operation needs gets 403. An unset
// secret matches no token, so a Worker without its secrets refuses everything that is not public.
package auth

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// Name is the security scheme's name in the specs.
const Name = "bearer"

// Token is one secret and the scopes it grants.
type Token struct {
	Secret string   // the Worker's secret (the environment variable) that holds the token
	Scopes []string // what it may do, e.g. read; write
}

// Scheme declares bearer tokens in the specs (config.OpenAPI), and the scopes an operation needs
// when it says nothing (none given: such operations are public). An operation that needs other
// scopes says so: Security: auth.Needs("write").
func Scheme(spec *huma.OpenAPI, scopes ...string) {
	if spec.Components == nil {
		spec.Components = &huma.Components{}
	}
	if spec.Components.SecuritySchemes == nil {
		spec.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	spec.Components.SecuritySchemes[Name] = &huma.SecurityScheme{Type: "http", Scheme: "bearer"}
	if len(scopes) > 0 {
		spec.Security = Needs(scopes...)
	}
}

// Needs is an operation's Security: a token with all of these scopes.
func Needs(scopes ...string) []map[string][]string {
	return []map[string][]string{{Name: append([]string{}, scopes...)}}
}

// Public is an operation's Security when it needs no token.
func Public() []map[string][]string { return []map[string][]string{} }

// Middleware enforces each operation's Security. secret looks a secret up (the Worker's
// environment); defaults are the API's default scopes, for an operation that declares none.
func Middleware(api huma.API, secret func(name string) string, tokens ...Token) func(huma.Context, func(huma.Context)) {
	// The API's document, for its default security: without registering routes when the API can
	// (humaworkers registers them as requests need them, after this middleware is in place).
	spec := api.OpenAPI
	if lazy, ok := api.(interface{ Spec() *huma.OpenAPI }); ok {
		spec = lazy.Spec
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
		granted, known := scopesOf(bearer(ctx.Header("Authorization")), secret, tokens)
		if !known {
			ctx.SetHeader("WWW-Authenticate", "Bearer")
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "a valid token is required: Authorization: Bearer <token>")
			return
		}
		// Security is a list of alternatives; the scopes of one are all needed.
		for _, alternative := range security {
			needs, ok := alternative[Name]
			if ok && !slices.ContainsFunc(needs, func(scope string) bool { return !slices.Contains(granted, scope) }) {
				next(ctx)
				return
			}
		}
		huma.WriteErr(api, ctx, http.StatusForbidden, "this token may not do this: it needs "+describe(security))
	}
}

// scopesOf are the scopes token grants, and whether it is one of the tokens at all. It compares
// with every secret in constant time.
func scopesOf(token string, secret func(string) string, tokens []Token) ([]string, bool) {
	var granted []string
	known := false
	for _, t := range tokens {
		want := ""
		if secret != nil {
			want = secret(t.Secret)
		}
		if want != "" && token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(want)) == 1 {
			known = true
			granted = append(granted, t.Scopes...)
		}
	}
	return granted, known
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
		alternatives = append(alternatives, "scope "+strings.Join(alternative[Name], " and "))
	}
	return strings.Join(alternatives, ", or ")
}
