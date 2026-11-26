package pages

import (
	"net/http"
	"strings"

	"github.com/joeblew999/charter/examples/site/cloud"
)

// serveCloud serves /cloud and its forms: the page of what Cloudflare gives the Worker (package cloud). A form is a
// plain post, answered with a redirect back to the page.
func serveCloud(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/cloud" && r.Method == http.MethodGet:
		state, err := cloud.Read(r)
		if err != nil {
			fail(w, err)
			return
		}
		render(w, r, http.StatusOK, Cloud(state))
	case r.Method == http.MethodPost:
		if err := cloud.Post(r); err != nil {
			fail(w, err)
			return
		}
		back(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// back sends the browser to the page it posted from: /cloud in its language.
func back(w http.ResponseWriter, r *http.Request) {
	to := "/cloud"
	if ref := r.Referer(); ref != "" {
		if i := strings.Index(ref, "://"); i >= 0 {
			if j := strings.Index(ref[i+3:], "/"); j >= 0 && strings.HasSuffix(strings.SplitN(ref[i+3+j:], "?", 2)[0], "/cloud") {
				to = strings.SplitN(ref[i+3+j:], "?", 2)[0]
			}
		}
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}
