package api

import (
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/asyncapi"
	"github.com/joeblew999/charter/go/auth"
	"github.com/joeblew999/charter/go/humaworkers"
)

func init() {
	// A list is [] when empty, never null: generated SDKs then type it as a plain array.
	huma.DefaultArrayNullable = false
}

func config() huma.Config {
	config := humaworkers.Config(Title, Version)
	config.Info.Description = Description
	// Bearer tokens; an operation that needs one says which scopes (reads are public).
	auth.Scheme(config.OpenAPI)
	return config
}

// OpenAPI is the OpenAPI spec of the contract, with server as its server: what cmd/spec writes
// for Fern and what the Worker serves at /api/openapi.json.
func OpenAPI(server string) ([]byte, error) {
	config := config()
	config.Servers = []*huma.Server{{URL: server}}
	return json.Marshal(humaworkers.New(config, Routes(Env{})).OpenAPI())
}

// AsyncAPI is the AsyncAPI spec of the contract's WebSocket channels (/api/asyncapi.json).
func AsyncAPI(server string) ([]byte, error) {
	api := humaworkers.New(config(), Routes(Env{}))
	doc, err := asyncapi.Generate(api.Operations(), api.OpenAPI().Components.Schemas, asyncapi.Info{Title: LiveTitle, Version: Version}, server)
	if err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}
