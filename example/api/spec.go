package api

import (
	"github.com/danielgtaylor/huma/v2"

	charter "github.com/joeblew999/charter/api"
)

func init() {
	// A list is [] when empty, never null: generated SDKs then type it as a plain array.
	huma.DefaultArrayNullable = false
}

func config() huma.Config {
	config := charter.HumaConfig(Title, Version)
	config.Info.Description = Description
	// Who may call: the app's own people (a token they made; on the pages, their session), bearer tokens, Cloudflare Access (a person's login or a machine's service token)
	// and an OpenID Connect issuer's tokens. An operation that needs a scope says which, by any of
	// them (reads are public).
	charter.Scheme(config.OpenAPI)
	charter.PeopleScheme(config.OpenAPI)
	charter.AccessScheme(config.OpenAPI, Title)
	charter.OIDCScheme(config.OpenAPI)
	// How often a caller may call: each limited operation says so, with its 429.
	charter.DeclareLimits(config.OpenAPI)
	return config
}

// API is the contract on env as charter/serve serves it: what the Worker serves (Handler), and the
// specs charter check --fix writes for Fern (API(Env{}).OpenAPI and .AsyncAPI).
func API(env Env) charter.API {
	return charter.API{Config: config, Routes: Routes(env), LiveTitle: LiveTitle, Setting: env.Var, Trusted: Trusted}
}
