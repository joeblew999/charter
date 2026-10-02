package asyncapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/orpc-api/api/go/humaworkers"
)

type event struct {
	ID   string `json:"id"`
	Body string `json:"body,omitempty"`
}

type subscribe struct {
	Topic string `json:"topic"`
}

type liveInput struct {
	After string `query:"after" doc:"Resume after this id"`
}

type subscribeInput struct {
	Body subscribe
}

// generate builds the document from routes, as a server's spec function does: from a humaworkers
// API's operations (Register has filled in their request bodies) and its schema registry.
func generate(t *testing.T, routes ...humaworkers.Route) (map[string]any, error) {
	t.Helper()
	api := humaworkers.New(humaworkers.Config("t", "1"), routes)
	doc, err := Generate(api.Operations(), api.OpenAPI().Components.Schemas, Info{Title: "live", Version: "1"}, "https://example.com")
	if err != nil {
		return nil, err
	}
	// Through JSON, as it is written: plain maps and lists.
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var plain map[string]any
	if err := json.Unmarshal(raw, &plain); err != nil {
		t.Fatal(err)
	}
	return plain, nil
}

func route[I, O any](op huma.Operation) humaworkers.Route {
	return humaworkers.Route{Method: op.Method, Path: op.Path, Register: func(api huma.API) {
		huma.Register(api, op, func(context.Context, *I) (*O, error) { return nil, nil })
	}}
}

var channel = route[liveInput, struct{}](Operation(huma.Operation{
	OperationID: "live", Method: http.MethodGet, Path: "/live", Summary: "Events",
}, Channel{Name: "live", Payload: event{}}))

func send[I any](send Send) humaworkers.Route {
	return route[I, struct{}](SendOperation(huma.Operation{OperationID: "subscribe", Method: http.MethodPost, Path: "/live"}, send))
}

func at(doc any, path ...string) any {
	for _, key := range path {
		m, ok := doc.(map[string]any)
		if !ok {
			return nil
		}
		doc = m[key]
	}
	return doc
}

func TestAChannelIsAReceiveOperationWithItsQueryAsTheBinding(t *testing.T) {
	doc, err := generate(t, channel)
	if err != nil {
		t.Fatal(err)
	}
	if at(doc, "channels", "live", "address") != "/live" || at(doc, "channels", "live", "summary") != "Events" {
		t.Errorf("channel: %v", at(doc, "channels", "live"))
	}
	if at(doc, "channels", "live", "bindings", "ws", "query", "properties", "after", "type") != "string" {
		t.Errorf("query binding: %v", at(doc, "channels", "live", "bindings"))
	}
	if at(doc, "operations", "receiveLive", "action") != "receive" || len(at(doc, "operations").(map[string]any)) != 1 {
		t.Errorf("operations: %v", at(doc, "operations"))
	}
	if at(doc, "components", "messages", "Event", "payload", "$ref") != "#/components/schemas/Event" || at(doc, "components", "schemas", "Event", "properties", "id") == nil {
		t.Errorf("components: %v", at(doc, "components"))
	}
}

func TestASendOperationAddsItsBodyAsAMessageOfTheChannel(t *testing.T) {
	doc, err := generate(t, channel, send[subscribeInput](Send{Channel: "live"}))
	if err != nil {
		t.Fatal(err)
	}
	messages, _ := at(doc, "channels", "live", "messages").(map[string]any)
	if len(messages) != 2 || at(messages, "Subscribe", "$ref") != "#/components/messages/Subscribe" || at(messages, "Event", "$ref") != "#/components/messages/Event" {
		t.Errorf("the channel's messages: %v", messages)
	}
	want := `{"action":"send","channel":{"$ref":"#/channels/live"},"messages":[{"$ref":"#/channels/live/messages/Subscribe"}]}`
	if got, _ := json.Marshal(at(doc, "operations", "sendSubscribe")); string(got) != want {
		t.Errorf("sendSubscribe: %s\nwant         %s", got, want)
	}
	if at(doc, "operations", "receiveLive", "action") != "receive" {
		t.Errorf("the receive operation is gone: %v", at(doc, "operations"))
	}
	if at(doc, "components", "messages", "Subscribe", "payload", "$ref") != "#/components/schemas/Subscribe" || at(doc, "components", "schemas", "Subscribe", "properties", "topic", "type") != "string" {
		t.Errorf("message Subscribe: %v", at(doc, "components"))
	}
}

func TestASendOperationCanNameItsOperationAndMessage(t *testing.T) {
	doc, err := generate(t, channel, send[subscribeInput](Send{Channel: "live", OperationID: "join", Message: "Join"}))
	if err != nil {
		t.Fatal(err)
	}
	if at(doc, "operations", "join", "action") != "send" || at(doc, "components", "messages", "Join", "name") != "Join" {
		t.Errorf("operations %v, messages %v", at(doc, "operations"), at(doc, "components", "messages"))
	}
}

func TestASendOperationNeedsItsChannelAndABody(t *testing.T) {
	_, err := generate(t, send[subscribeInput](Send{Channel: "live"}))
	if err == nil || !strings.Contains(err.Error(), "not a channel") {
		t.Errorf("without its channel: %v", err)
	}
	_, err = generate(t, channel, send[struct{}](Send{Channel: "live"}))
	if err == nil || !strings.Contains(err.Error(), "needs a JSON body") {
		t.Errorf("without a body: %v", err)
	}
}

func TestBothAreHiddenFromOpenAPI(t *testing.T) {
	if !Operation(huma.Operation{}, Channel{}).Hidden || !SendOperation(huma.Operation{}, Send{}).Hidden {
		t.Error("a channel or a send operation is not hidden")
	}
	if !Is(&huma.Operation{Metadata: Operation(huma.Operation{}, Channel{}).Metadata}) || Is(&huma.Operation{Metadata: SendOperation(huma.Operation{}, Send{}).Metadata}) {
		t.Error("Is is for channels only")
	}
}
