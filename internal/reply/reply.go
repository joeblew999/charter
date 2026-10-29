// Package reply writes a JSON answer, for what the library answers outside Huma: MCP's endpoint and the
// documents at /.well-known/.
package reply

import (
	"encoding/json"
	"net/http"
)

// JSON writes body as the answer, JSON, with status.
func JSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data)
}
