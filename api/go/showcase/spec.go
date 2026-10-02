package showcase

import (
	"encoding/json"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/orpc-api/api/go/asyncapi"
	"github.com/joeblew999/orpc-api/api/go/humaworkers"
)

func init() {
	// A list is [] when empty, never null: generated SDKs then type it as a plain array.
	huma.DefaultArrayNullable = false
}

// config is the document around the operations: the parts of the spec that belong to no single
// operation. Huma's OpenAPI struct has a field for each, so they are said here, in code.
func config() huma.Config {
	config := humaworkers.WithForm(humaworkers.Config(Title, Version))
	config.Info.Description = Description
	// OAuth client credentials, required everywhere unless an operation says otherwise.
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{OAuth: {
		Type:  "oauth2",
		Flows: &huma.OAuthFlows{ClientCredentials: &huma.OAuthFlow{TokenURL: "/oauth/token", Scopes: map[string]string{}}},
	}}
	config.Security = []map[string][]string{{OAuth: {}}}
	config.Extensions = map[string]any{
		// The header the SDKs send for an operation marked x-fern-idempotent, and the option's name.
		"x-fern-idempotency-headers": []map[string]string{{"header": "Idempotency-Key", "name": "idempotency_key"}},
		// How webhooks are signed: the SDKs get a helper that verifies it (what notify sends).
		"x-fern-webhook-signature": map[string]string{"type": "hmac", "header": SignatureHeader, "algorithm": "sha256", "encoding": "hex"},
	}
	return config
}

// document is the whole API registered, with server as its server, plus the webhooks: what the
// server sends, which no route describes. Huma has the field but builds nothing for it, so the
// operation is written out here, with the payload's schema from the struct notify sends.
func document(server string) *humaworkers.API {
	config := config()
	config.Servers = []*huma.Server{{URL: server}}
	api := humaworkers.New(config, Routes(Env{}))
	doc := api.OpenAPI()
	doc.Webhooks = map[string]*huma.PathItem{"noteCreated": {Post: &huma.Operation{
		OperationID: "noteCreatedWebhook",
		Summary:     "Sent to you when a note is created",
		Tags:        []string{"notes"},
		RequestBody: &huma.RequestBody{Content: map[string]*huma.MediaType{
			"application/json": {Schema: doc.Components.Schemas.Schema(reflect.TypeFor[NoteCreated](), true, "")},
		}},
		Responses: map[string]*huma.Response{"200": {Description: "received"}},
	}}}
	return api
}

// OpenAPI is the OpenAPI spec of the contract, with server as its server: what cmd/spec writes
// for Fern and what the server gives at /openapi.json.
func OpenAPI(server string) ([]byte, error) {
	return json.Marshal(document(server).OpenAPI())
}

// AsyncAPI is the AsyncAPI spec of the contract's WebSocket channel (/asyncapi.json).
func AsyncAPI(server string) ([]byte, error) {
	api := document(server)
	doc, err := asyncapi.Generate(api.Operations(), api.OpenAPI().Components.Schemas, asyncapi.Info{Title: LiveTitle, Version: Version}, server)
	if err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}
