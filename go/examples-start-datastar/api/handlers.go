package api

import (
	"context"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/follow"
	"github.com/joeblew999/charter/go/humamcp"
	"github.com/joeblew999/charter/go/humaworkers"
)

// Env is what the platform supplies: bindings on Cloudflare (platform_js.go), memory elsewhere
// (platform_other.go). Store and Hub are opened per request: a binding belongs to the request's
// environment.
type Env struct {
	Var   func(name string) string
	Store func() (Store, error)
	Hub   func() (Hub, error)
}

// Handler serves the contract on env, plus the two specs with the request's origin as their server,
// plus the contract as MCP tools (/api/mcp: a tool call runs the same operation as the REST route).
func Handler(env Env) http.Handler {
	routes := humaworkers.New(config(), Routes(env))
	mcp := humamcp.Handler(routes)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var spec func(server string) ([]byte, error)
		switch r.URL.Path {
		case "/api/openapi.json":
			spec = OpenAPI
		case "/api/asyncapi.json":
			spec = AsyncAPI
		case "/api/mcp":
			mcp.ServeHTTP(w, r)
			return
		default:
			routes.ServeHTTP(w, r)
			return
		}
		body, err := spec(origin(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})
}

// origin is the request's scheme and host: absolute in r.URL on Workers, from Host on net/http.
func origin(r *http.Request) string {
	if r.URL.Scheme != "" && r.URL.Host != "" {
		return r.URL.Scheme + "://" + r.URL.Host
	}
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

func (env Env) hello(context.Context, *struct{}) (*HelloOutput, error) {
	out := &HelloOutput{}
	out.Body.Message = "Hello from " + env.Var("APP_NAME")
	return out, nil
}

func (env Env) list(ctx context.Context, in *ListInput) (*ListOutput, error) {
	messages, err := env.Recent(ctx, int(in.Limit))
	if err != nil {
		return nil, err
	}
	out := &ListOutput{}
	out.Body.Data = messages
	return out, nil
}

func (env Env) create(ctx context.Context, in *CreateInput) (*MessageOutput, error) {
	message, err := env.Post(ctx, in.Body.Body)
	if errors.Is(err, ErrBody) {
		return nil, huma.Error422UnprocessableEntity("validation failed", &huma.ErrorDetail{Location: "body.body", Message: err.Error(), Value: in.Body.Body})
	}
	if err != nil {
		return nil, err
	}
	return &MessageOutput{Body: message}, nil
}

// ErrBody is what Post says of a message that is empty or longer than MaxBody.
var ErrBody = errors.New("a message is 1 to " + strconv.Itoa(MaxBody) + " characters, not only spaces")

// Post stores a message and announces it: what POST /api/messages and the pages' form both do.
func (env Env) Post(ctx context.Context, body string) (Message, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > MaxBody {
		return Message{}, ErrBody
	}
	store, err := env.Store()
	if err != nil {
		return Message{}, err
	}
	message, err := store.Create(ctx, body)
	if err != nil {
		return Message{}, err
	}
	// The hub is only a wake-up: if it misses this message, followers find it in the log (Recheck).
	if hub, err := env.Hub(); err != nil {
		log.Printf("post: no hub, message %d is not announced: %v", message.ID, err)
	} else if err := hub.Publish(ctx, message); err != nil {
		log.Printf("post: message %d is not announced: %v", message.ID, err)
	}
	return message, nil
}

// Recent is the newest messages, newest first, at most limit.
func (env Env) Recent(ctx context.Context, limit int) ([]Message, error) {
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	return store.Before(ctx, math.MaxInt64, limit)
}

// Feed is the log and its wake-up together: what go/follow follows. The store is the log, the
// hub only wakes followers, so a stream loses nothing when the hub does.
type Feed struct {
	store Store
	hub   Hub
}

func (f Feed) Subscribe(listener func(Message), onError func(error)) (func(), error) {
	return f.hub.Subscribe(listener, onError)
}
func (f Feed) Since(ctx context.Context, after int64, limit int) ([]Message, error) {
	return f.store.Since(ctx, after, limit)
}
func (f Feed) Latest(ctx context.Context) (int64, error) { return f.store.Latest(ctx) }

// Feed opens the feed and reads the resume position: the newest of the positions given (the page's
// `after`, and the Last-Event-ID a reconnecting browser sends). None: from now on.
func (env Env) Feed(positions ...string) (Feed, follow.Options, error) {
	options := follow.Options{OnBroken: func(err error) { log.Printf("follow: hub subscription broken, resubscribing: %v", err) }}
	for _, position := range positions {
		id, err := strconv.ParseInt(position, 10, 64)
		if err != nil || id < 0 {
			continue // absent, or not a message id
		}
		if options.After == nil || id > *options.After {
			options.After = &id
		}
	}
	store, err := env.Store()
	if err != nil {
		return Feed{}, options, err
	}
	hub, err := env.Hub()
	if err != nil {
		return Feed{}, options, err
	}
	return Feed{store, hub}, options, nil
}
