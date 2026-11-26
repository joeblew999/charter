// WriteProblem writes a refusal as RFC 9457 problem details, as Huma writes an operation's
// (huma.ErrorModel), for what answers outside an operation: an unknown path (404), a method the path
// lacks (405), /.well-known/. Every refusal of an API is then the same kind of answer
// (docs/self/standard.md), and the same as charter-lib's (problem.ts).

package humaworkers

import (
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// mediaType is the media type of a problem.
const mediaType = "application/problem+json"

// WriteProblem answers w with status and what went wrong.
func WriteProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", mediaType)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(&huma.ErrorModel{Title: http.StatusText(status), Status: status, Detail: detail})
}
