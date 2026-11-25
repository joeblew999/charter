//go:build !(js && wasm)

package pages

import "net/http"

// Off Workers (go test, go run .) there are no bindings of workers-go's: the page renders and says so.

// Background does nothing off Workers.
func Background() {}

func cloud(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/cloud" || r.Method != http.MethodGet {
		http.Error(w, "the Cloudflare page's forms need a Worker", http.StatusNotImplemented)
		return
	}
	render(w, r, http.StatusOK, Cloud(CloudState{}))
}
