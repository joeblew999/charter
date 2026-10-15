package api

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/lib/auth"
	"github.com/joeblew999/charter/go/lib/humaworkers"
	"github.com/joeblew999/charter/go/lib/ratelimit"
	"github.com/joeblew999/charter/go/lib/serve"
)

func init() {
	// A list is [] when empty, never null: generated SDKs then type it as a plain array.
	huma.DefaultArrayNullable = false
}

func config() huma.Config {
	config := humaworkers.Config(Title, Version)
	config.Info.Description = Description
	// Who may call: bearer tokens, Cloudflare Access (a person's login or a machine's service token)
	// and an OpenID Connect issuer's tokens. An operation that needs a scope says which, by any of
	// them (reads are public).
	auth.Scheme(config.OpenAPI)
	auth.AccessScheme(config.OpenAPI, Title)
	auth.OIDCScheme(config.OpenAPI)
	// How often a caller may call: each limited operation says so, with its 429.
	ratelimit.Declare(config.OpenAPI)
	return config
}

// API is the contract on env as go/lib/serve serves it: what the Worker serves (Handler), and the
// specs npm run spec writes for Fern (API(Env{}).OpenAPI and .AsyncAPI).
func API(env Env) serve.API {
	return serve.API{Config: config, Routes: Routes(env), LiveTitle: LiveTitle, Setting: env.Var, Trusted: Trusted}
}
