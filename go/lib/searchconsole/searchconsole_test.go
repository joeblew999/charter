package searchconsole_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/joeblew999/charter/go/lib/searchconsole"
)

// fakeGoogle is a token endpoint that verifies the JWT assertion's RS256 signature, plus the Search Console endpoints.
func fakeGoogle(t *testing.T, pub *rsa.PublicKey) (*httptest.Server, *[]string) {
	var calls []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		parts := strings.Split(r.Form.Get("assertion"), ".")
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || len(parts) != 3 {
			http.Error(w, `{"error_description":"bad grant"}`, 400)
			return
		}
		sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
		sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig) != nil {
			http.Error(w, `{"error_description":"bad signature"}`, 400)
			return
		}
		claims, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var c map[string]any
		json.Unmarshal(claims, &c)
		if c["iss"] != "sa@proj.iam.gserviceaccount.com" || c["scope"] != searchconsole.Scope {
			http.Error(w, `{"error_description":"bad claims"}`, 400)
			return
		}
		io.WriteString(w, `{"access_token":"tok","expires_in":3600}`)
	})
	api := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, `{"error":{"message":"unauthenticated"}}`, 401)
			return
		}
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, r.Method+" "+r.URL.EscapedPath()+" "+string(body))
		switch {
		case r.URL.Path == "/webmasters/v3/sites":
			io.WriteString(w, `{"siteEntry":[{"siteUrl":"sc-domain:example.com","permissionLevel":"siteFullUser"}]}`)
		case r.Method == "PUT":
			w.WriteHeader(204)
		case strings.Contains(r.URL.Path, "/sitemaps/"):
			io.WriteString(w, `{"path":"https://app.example.com/sitemap.xml","isPending":false,"errors":"0","contents":[{"type":"web","submitted":"56","indexed":"0"}]}`)
		case r.URL.Path == "/v1/urlInspection/index:inspect":
			io.WriteString(w, `{"inspectionResult":{"indexStatusResult":{"verdict":"NEUTRAL","coverageState":"URL is unknown to Google"}}}`)
		default:
			http.NotFound(w, r)
		}
	}
	mux.HandleFunc("/webmasters/v3/", api)
	mux.HandleFunc("/v1/", api)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestSearchConsole(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	srv, calls := fakeGoogle(t, &priv.PublicKey)
	keyJSON, _ := json.Marshal(map[string]string{
		"client_email": "sa@proj.iam.gserviceaccount.com",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"token_uri":    srv.URL + "/token",
	})
	key, err := searchconsole.ParseKey(keyJSON)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tok, err := key.Token(ctx, srv.Client(), searchconsole.Scope)
	if err != nil || tok != "tok" {
		t.Fatalf("Token = %q, %v", tok, err)
	}
	c := &searchconsole.Client{HTTP: srv.Client(), Token: tok, Site: "sc-domain:example.com",
		WebmastersBase: srv.URL + "/webmasters/v3", SearchConsoleBase: srv.URL + "/v1"}

	sites, err := c.Sites(ctx)
	if err != nil || len(sites) != 1 || sites[0].PermissionLevel != "siteFullUser" {
		t.Errorf("Sites = %+v, %v", sites, err)
	}
	feed := "https://app.example.com/sitemap.xml"
	if err := c.SubmitSitemap(ctx, feed); err != nil {
		t.Errorf("SubmitSitemap: %v", err)
	}
	sm, err := c.GetSitemap(ctx, feed)
	if err != nil || len(sm.Contents) != 1 || sm.Contents[0].Submitted != "56" {
		t.Errorf("GetSitemap = %+v, %v", sm, err)
	}
	in, err := c.Inspect(ctx, "https://app.example.com/de/", "de")
	if err != nil || in.CoverageState != "URL is unknown to Google" {
		t.Errorf("Inspect = %+v, %v", in, err)
	}
	wantPut := "PUT /webmasters/v3/sites/" + url.PathEscape("sc-domain:example.com") + "/sitemaps/" + url.PathEscape(feed) + " "
	if len(*calls) < 4 || (*calls)[1] != wantPut || !strings.Contains((*calls)[3], `"siteUrl":"sc-domain:example.com"`) {
		t.Errorf("calls = %q", *calls)
	}

	bad := &searchconsole.Client{HTTP: srv.Client(), Token: "nope", WebmastersBase: srv.URL + "/webmasters/v3"}
	if _, err := bad.Sites(ctx); err == nil || !strings.Contains(err.Error(), "unauthenticated") {
		t.Errorf("bad token: %v", err)
	}
}

func TestSnippetWidth(t *testing.T) {
	for s, want := range map[string]int{"": 0, "Home": 4, "Café": 4, "ホーム": 6, "服务器，": 8, "gsx 组件": 8, "مكونات": 6} {
		if got := searchconsole.SnippetWidth(s); got != want {
			t.Errorf("SnippetWidth(%q) = %d, want %d", s, got, want)
		}
	}
}
