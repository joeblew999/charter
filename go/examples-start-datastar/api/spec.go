package api

import (
	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/lib/humaworkers"
	"github.com/joeblew999/charter/go/lib/serve"
)

func init() {
	// A list is [] when empty, never null: generated SDKs then type it as a plain array.
	huma.DefaultArrayNullable = false
}

func config() huma.Config {
	config := humaworkers.Config(Title, Version)
	config.Info.Description = Description
	return config
}

// API is the contract on env as go/lib/serve serves it: what the Worker serves (Handler), and the
// specs cmd/spec writes for Fern (API(Env{}).OpenAPI and .AsyncAPI).
func API(env Env) serve.API {
	return serve.API{Config: config, Routes: Routes(env), LiveTitle: LiveTitle, Setting: env.Var}
}
