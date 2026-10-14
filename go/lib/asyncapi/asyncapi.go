// Package asyncapi generates an AsyncAPI 3.0.0 document from a Huma API, the way Huma itself
// generates OpenAPI: the Go port of ts/src/asyncapi.ts.
//
// An operation marked with Channel is a WebSocket channel:
//   - its query parameters (the input struct's `query:"..."` fields) become the channel's
//     bindings.ws.query;
//   - Channel.Payload (a value of the message type) becomes the payload of a `receive` operation.
//
// An operation marked with SendOperation is a message the client sends on a channel: its JSON
// request body becomes the payload of a `send` operation (Fern's TypeScript client then has a typed
// send<Message>() on the socket). It is `send: { message, operationId }` of ts/src/asyncapi.ts, and
// the document comes out the same. There the message is the channel procedure's input stream; a
// Huma operation takes one request, so here it is an operation of its own, the one that handles
// the message. A channel can so have query parameters and a send side together.
//
// Both are hidden from OpenAPI: Fern reads WebSockets from AsyncAPI only.
package asyncapi

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// Channel describes one WebSocket channel.
type Channel struct {
	// Name is the channel id; Fern names the client after it (liveNotes -> client.liveNotes.connect()).
	Name string
	// OperationID of the receive operation. Default: receive<Name>.
	OperationID string
	// Message is the message's name in components.messages. Default: the payload's schema name.
	Message string
	// Payload is a value of the type every message carries, e.g. Note{}.
	Payload any
	// Extensions are added to the channel object (x-fern-* and the like).
	Extensions map[string]any
}

// Send describes a message the client sends on a channel.
type Send struct {
	// Channel is the Name of the channel it is sent on.
	Channel string
	// OperationID of the send operation. Default: send<Message>.
	OperationID string
	// Message is the message's name in components.messages. Default: the body's schema name.
	Message string
}

const (
	metadataKey     = "asyncapi"
	sendMetadataKey = "asyncapi.send"
)

// Operation marks op as a channel: its Path is the channel's address, its Summary and Description
// the channel's.
func Operation(op huma.Operation, channel Channel) huma.Operation {
	op.Hidden = true
	if op.Metadata == nil {
		op.Metadata = map[string]any{}
	}
	op.Metadata[metadataKey] = channel
	return op
}

// SendOperation marks op as a message the client sends on a channel: its JSON request body (the
// input struct's Body) is the message's payload. How the message reaches the operation is the
// transport's business (api/go/transport gives each frame to a POST on the channel's path).
func SendOperation(op huma.Operation, send Send) huma.Operation {
	op.Hidden = true
	if op.Metadata == nil {
		op.Metadata = map[string]any{}
	}
	op.Metadata[sendMetadataKey] = send
	return op
}

// Is reports whether op is a channel.
func Is(op *huma.Operation) bool {
	_, ok := op.Metadata[metadataKey].(Channel)
	return ok
}

// Info is the document's info object.
type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// Generate builds the document for the channels among ops (humaworkers' API.Operations()), with
// schemas from registry (api.OpenAPI().Components.Schemas). server is the API's URL: its host, and
// ws or wss from its scheme.
func Generate(ops []*huma.Operation, registry huma.Registry, info Info, server string) (map[string]any, error) {
	scheme, host, ok := strings.Cut(server, "://")
	if !ok || host == "" {
		return nil, fmt.Errorf("asyncapi: server %q is not a URL", server)
	}
	protocol := "wss"
	if scheme == "http" {
		protocol = "ws"
	}
	channels, operations, messages := map[string]any{}, map[string]any{}, map[string]any{}
	used := map[string]bool{}
	for _, op := range ops {
		channel, ok := op.Metadata[metadataKey].(Channel)
		if !ok {
			continue
		}
		if channel.Name == "" || channel.Payload == nil {
			return nil, fmt.Errorf("asyncapi: %s %s: a channel needs a Name and a Payload", op.Method, op.Path)
		}
		payload := registry.Schema(reflect.TypeOf(channel.Payload), true, channel.Name+"Message")
		message := channel.Message
		if message == "" {
			message = strings.TrimPrefix(payload.Ref, "#/components/schemas/")
		}
		if message == "" {
			message = channel.Name
		}
		collect(registry, payload, used)
		messages[message] = map[string]any{"name": message, "payload": payload}

		entry := map[string]any{
			"address":  op.Path,
			"messages": map[string]any{message: map[string]any{"$ref": "#/components/messages/" + message}},
		}
		if op.Summary != "" {
			entry["summary"] = op.Summary
		}
		if op.Description != "" {
			entry["description"] = op.Description
		}
		if query := queryObject(op); query != nil {
			for _, param := range op.Parameters {
				if param.In == "query" {
					collect(registry, param.Schema, used)
				}
			}
			entry["bindings"] = map[string]any{"ws": map[string]any{"query": query}}
		}
		for key, value := range channel.Extensions {
			entry[key] = value
		}
		channels[channel.Name] = entry

		operationID := channel.OperationID
		if operationID == "" {
			operationID = "receive" + strings.ToUpper(channel.Name[:1]) + channel.Name[1:]
		}
		operations[operationID] = map[string]any{
			"action":   "receive",
			"channel":  map[string]any{"$ref": "#/channels/" + channel.Name},
			"messages": []any{map[string]any{"$ref": "#/channels/" + channel.Name + "/messages/" + message}},
		}
	}
	for _, op := range ops {
		send, ok := op.Metadata[sendMetadataKey].(Send)
		if !ok {
			continue
		}
		channel, ok := channels[send.Channel].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("asyncapi: %s %s: sends on %q, which is not a channel", op.Method, op.Path, send.Channel)
		}
		var payload *huma.Schema
		if op.RequestBody != nil && op.RequestBody.Content["application/json"] != nil {
			payload = op.RequestBody.Content["application/json"].Schema
		}
		if payload == nil {
			return nil, fmt.Errorf("asyncapi: %s %s: a send operation needs a JSON body, the message", op.Method, op.Path)
		}
		message := send.Message
		if message == "" {
			message = strings.TrimPrefix(payload.Ref, "#/components/schemas/")
		}
		if message == "" {
			return nil, fmt.Errorf("asyncapi: %s %s: name the message (Send.Message): its body has no schema name", op.Method, op.Path)
		}
		collect(registry, payload, used)
		messages[message] = map[string]any{"name": message, "payload": payload}
		channel["messages"].(map[string]any)[message] = map[string]any{"$ref": "#/components/messages/" + message}

		operationID := send.OperationID
		if operationID == "" {
			operationID = "send" + strings.ToUpper(message[:1]) + message[1:]
		}
		operations[operationID] = map[string]any{
			"action":   "send",
			"channel":  map[string]any{"$ref": "#/channels/" + send.Channel},
			"messages": []any{map[string]any{"$ref": "#/channels/" + send.Channel + "/messages/" + message}},
		}
	}
	components := map[string]any{"messages": messages}
	if len(used) > 0 {
		schemas := map[string]any{}
		for name := range used {
			schemas[name] = registry.Map()[name]
		}
		components["schemas"] = schemas
	}
	return map[string]any{
		"asyncapi":   "3.0.0",
		"info":       info,
		"servers":    map[string]any{"production": map[string]any{"host": strings.TrimSuffix(host, "/"), "protocol": protocol}},
		"channels":   channels,
		"operations": operations,
		"components": components,
	}, nil
}

// queryObject is the operation's query parameters as one object schema, or nil when it has none.
func queryObject(op *huma.Operation) map[string]any {
	properties := map[string]any{}
	var required []string
	for _, param := range op.Parameters {
		if param.In != "query" {
			continue
		}
		properties[param.Name] = param.Schema
		if param.Required {
			required = append(required, param.Name)
		}
	}
	if len(properties) == 0 {
		return nil
	}
	query := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		query["required"] = required
	}
	return query
}

// collect notes every registry schema that s refers to, directly or through others.
func collect(registry huma.Registry, s *huma.Schema, used map[string]bool) {
	if s == nil {
		return
	}
	if name := strings.TrimPrefix(s.Ref, "#/components/schemas/"); s.Ref != "" && !used[name] {
		used[name] = true
		collect(registry, registry.Map()[name], used)
	}
	collect(registry, s.Items, used)
	collect(registry, s.Not, used)
	if child, ok := s.AdditionalProperties.(*huma.Schema); ok {
		collect(registry, child, used)
	}
	for _, child := range s.Properties {
		collect(registry, child, used)
	}
	for _, list := range [][]*huma.Schema{s.OneOf, s.AnyOf, s.AllOf} {
		for _, child := range list {
			collect(registry, child, used)
		}
	}
}
