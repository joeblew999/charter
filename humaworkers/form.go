package humaworkers

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
)

// FormContentType is the media type of an HTML form's body, and of an OAuth token request.
const FormContentType = "application/x-www-form-urlencoded"

// WithForm lets operations take form-encoded request bodies, which Huma has no format for. Tag the
// input's Body with `contentType:"application/x-www-form-urlencoded"`: the spec then names that
// media type, and the body is validated and decoded like a JSON one. Every form value is a string
// (a repeated name is a list of strings), so the Body's fields must be strings too.
func WithForm(config huma.Config) huma.Config {
	formats := map[string]huma.Format{}
	for name, format := range config.Formats {
		formats[name] = format
	}
	formats[FormContentType] = huma.Format{
		Marshal: func(io.Writer, any) error {
			return errors.New("humaworkers: " + FormContentType + " is for request bodies only")
		},
		Unmarshal: unmarshalForm,
	}
	config.Formats = formats
	return config
}

// unmarshalForm decodes a form body the way Huma asks a format to: first into an `any`, which it
// validates against the schema, then into the Body struct (through its json names).
func unmarshalForm(data []byte, v any) error {
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return err
	}
	fields := make(map[string]any, len(values))
	for name, list := range values {
		if len(list) == 1 {
			fields[name] = list[0]
			continue
		}
		items := make([]any, len(list))
		for i, item := range list {
			items[i] = item
		}
		fields[name] = items
	}
	if target, ok := v.(*any); ok {
		*target = fields
		return nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, v)
}
