package api

import (
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/auth"
	"github.com/joeblew999/charter/go/humaworkers"
	"github.com/joeblew999/charter/go/ratelimit"
	"github.com/joeblew999/charter/go/serve"
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
	// Four ways in, any of them: OAuth client credentials (the SDKs fetch the token themselves), and
	// charter's three (go/lib/auth): the API's own tokens, Cloudflare Access and an OpenID Connect
	// issuer. Reading needs read and writing write, by any of them: an OAuth token has both.
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{OAuth: {
		Type:  "oauth2",
		Flows: &huma.OAuthFlows{ClientCredentials: &huma.OAuthFlow{TokenURL: "/api/oauth/token", Scopes: map[string]string{"read": "Read notes", "write": "Create notes and upload files"}}},
	}}
	auth.Scheme(config.OpenAPI, "read")
	auth.AccessScheme(config.OpenAPI, Title)
	auth.OIDCScheme(config.OpenAPI)
	config.Security = append(config.Security, map[string][]string{OAuth: {"read"}})
	// Each limited operation's limit and its 429, in the specs (createNote: WriteLimit).
	ratelimit.Declare(config.OpenAPI)
	config.Extensions = map[string]any{
		// The header the SDKs send for an operation marked x-fern-idempotent, and the option's name.
		"x-fern-idempotency-headers": []map[string]string{{"header": "Idempotency-Key", "name": "idempotency_key"}},
		// How webhooks are signed: the SDKs get a helper that verifies it (what notify sends).
		"x-fern-webhook-signature": map[string]string{"type": "hmac", "header": SignatureHeader, "algorithm": "sha256", "encoding": "hex"},
	}
	return config
}

// API is the contract on env as go/lib/serve serves it: what the server serves (Handler), and the
// specs cmd/spec writes for Fern (API(Env{}).OpenAPI and .AsyncAPI).
func API(env Env) serve.API {
	return serve.API{Config: config, Routes: Routes(env), LiveTitle: LiveTitle, Setting: env.setting, Trusted: env.trusted(), Document: webhooks}
}

// webhooks are what the server sends, which no route describes. Huma has the field but builds
// nothing for it, so the operation is written out here, with the payload's schema from the struct
// notify sends.
func webhooks(api *humaworkers.API) {
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
}
