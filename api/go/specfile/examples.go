package specfile

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/orpc-api/api/go/humaworkers"
)

// Examples is what is wrong with the examples in api's contract (Huma's `example:"..."` tag on a
// field). Fern puts an example of every request into each SDK's README, reference and tests, and
// invents the values the contract doesn't give: "id" for an id, whatever its pattern says. Someone
// who copies that gets a 422. So:
//
//   - a request field with a constraint (pattern, enum, minimum, maximum, minLength, maxLength)
//     must have an example, its own or as part of the example of what it is in (a list's example
//     gives its items');
//   - every example, in requests and responses, must be valid against its own schema.
//
// Hidden operations are checked too: a WebSocket channel's parameters are in the AsyncAPI spec.
// A contract's tests call it (api/contract_test.go).
func Examples(api *humaworkers.API) []error {
	ops := api.Operations()
	e := examples{registry: api.API.OpenAPI().Components.Schemas, seen: map[seen]bool{}, said: map[string]bool{}}
	for _, op := range ops {
		for _, param := range op.Parameters {
			if param == nil || param.Schema == nil {
				continue
			}
			where := op.OperationID + ": " + param.In + "." + param.Name
			given := e.walk(param.Schema, where, true, param.Example != nil)
			// A parameter's example may also be said on the parameter, beside its schema.
			if param.Example != nil {
				e.validate(param.Schema, param.Example, where, true)
			} else if !given && constrained(param.Schema) {
				e.missing(where)
			}
		}
		if op.RequestBody != nil {
			for _, name := range sorted(op.RequestBody.Content) {
				e.body(op.RequestBody.Content[name], op.OperationID+": body", true)
			}
		}
		for _, status := range sorted(op.Responses) {
			if response := op.Responses[status]; response != nil {
				for _, name := range sorted(response.Content) {
					e.body(response.Content[name], op.OperationID+": "+status+" response", false)
				}
			}
		}
	}
	return e.problems
}

type examples struct {
	registry huma.Registry
	seen     map[seen]bool
	said     map[string]bool
	problems []error
}

// seen is a schema checked as part of a request or of a response, inside something with an example
// or not: a named schema is walked once each way.
type seen struct {
	schema           *huma.Schema
	request, covered bool
}

func (e *examples) body(media *huma.MediaType, where string, request bool) {
	if media == nil || media.Schema == nil {
		return
	}
	if given := e.walk(media.Schema, where, request, false); !given && request && constrained(e.resolve(media.Schema)) {
		e.missing(where)
	}
}

// walk checks schema's examples and those of everything in it, and reports whether schema has one.
// A constrained request field in it without an example is a problem, unless schema has one or is
// itself inside something that has (covered); schema itself is the caller's.
func (e *examples) walk(schema *huma.Schema, where string, request, covered bool) bool {
	schema = e.resolve(schema)
	if schema == nil {
		return false
	}
	covered = covered || len(schema.Examples) > 0
	if key := (seen{schema, request, covered}); !e.seen[key] {
		e.seen[key] = true
		for _, example := range schema.Examples {
			e.validate(schema, example, where, request)
		}
		inside := func(child *huma.Schema, where string) {
			if given := e.walk(child, where, request, covered); !given && !covered && request && constrained(e.resolve(child)) {
				e.missing(where)
			}
		}
		for _, name := range sorted(schema.Properties) {
			inside(schema.Properties[name], where+"."+name)
		}
		if schema.Items != nil {
			inside(schema.Items, where+"[]")
		}
		for _, list := range [][]*huma.Schema{schema.OneOf, schema.AnyOf, schema.AllOf} {
			for _, child := range list {
				inside(child, where)
			}
		}
	}
	return len(schema.Examples) > 0
}

func (e *examples) resolve(schema *huma.Schema) *huma.Schema {
	for schema != nil && schema.Ref != "" {
		schema = e.registry.SchemaFromRef(schema.Ref)
	}
	return schema
}

func (e *examples) missing(where string) {
	e.problem(fmt.Errorf(`%s is constrained and has no example: add example:"..." to the field`, where))
}

// problem adds one, once.
func (e *examples) problem(err error) {
	if !e.said[err.Error()] {
		e.said[err.Error()] = true
		e.problems = append(e.problems, err)
	}
}

// validate checks one example as Huma would check it in a request (or, in a response, as a client
// would read it): as the JSON it is in the spec.
func (e *examples) validate(schema *huma.Schema, example any, where string, request bool) {
	var value any
	data, err := json.Marshal(example)
	if err == nil {
		err = json.Unmarshal(data, &value)
	}
	if err != nil {
		e.problem(fmt.Errorf("%s: example %v: %v", where, example, err))
		return
	}
	mode := huma.ModeReadFromServer
	if request {
		mode = huma.ModeWriteToServer
	}
	result := &huma.ValidateResult{}
	huma.Validate(e.registry, e.resolve(schema), huma.NewPathBuffer([]byte{}, 0), mode, value, result)
	for _, problem := range result.Errors {
		e.problem(fmt.Errorf("%s: example %s is not valid: %v", where, data, problem))
	}
}

// constrained reports whether a value made up for schema's type could be refused.
func constrained(s *huma.Schema) bool {
	return s != nil && (s.Pattern != "" || len(s.Enum) > 0 || s.Minimum != nil || s.Maximum != nil ||
		s.ExclusiveMinimum != nil || s.ExclusiveMaximum != nil || s.MinLength != nil || s.MaxLength != nil)
}

func sorted[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
