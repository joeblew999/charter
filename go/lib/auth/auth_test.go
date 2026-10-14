package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/joeblew999/charter/go/auth/authtest"
)

type out struct{ Body string }

// An API with a public operation, one that takes the default scope (read) and one that needs write.
// The handlers answer with who called.
func testAPI(t *testing.T, settings map[string]string, trusted ...Trusted) humatest.TestAPI {
	_, api := humatest.New(t)
	Scheme(api.OpenAPI(), "read")
	AccessScheme(api.OpenAPI(), "notes-test")
	OIDCScheme(api.OpenAPI())
	if trusted == nil {
		trusted = []Trusted{
			Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}},
			Token{Secret: "READ_TOKEN", Scopes: []string{"read"}},
		}
	}
	api.UseMiddleware(Middleware(api, func(name string) string { return settings[name] }, trusted...))
	handler := func(ctx context.Context, _ *struct{}) (*out, error) {
		c, _ := CallerOf(ctx)
		return &out{Body: c.Name()}, nil
	}
	huma.Register(api, huma.Operation{OperationID: "hello", Method: http.MethodGet, Path: "/hello", Security: Public()}, handler)
	huma.Register(api, huma.Operation{OperationID: "list", Method: http.MethodGet, Path: "/notes"}, handler)
	huma.Register(api, huma.Operation{OperationID: "create", Method: http.MethodPost, Path: "/notes", Security: Needs("write")}, handler)
	return api
}

func TestScopes(t *testing.T) {
	api := testAPI(t, map[string]string{"READ_TOKEN": "r", "WRITE_TOKEN": "w"})
	for _, c := range []struct {
		method, path, token string
		want                int
	}{
		{"GET", "/hello", "", 200},
		{"GET", "/notes", "", 401},
		{"GET", "/notes", "wrong", 401},
		{"GET", "/notes", "r", 200},
		{"GET", "/notes", "w", 200},
		{"POST", "/notes", "", 401},
		{"POST", "/notes", "r", 403},
		{"POST", "/notes", "w", 200},
	} {
		var args []any
		if c.token != "" {
			args = append(args, "Authorization: Bearer "+c.token)
		}
		resp := api.Do(c.method, c.path, args...)
		if resp.Code != c.want {
			t.Errorf("%s %s with %q: %d, want %d: %s", c.method, c.path, c.token, resp.Code, c.want, resp.Body)
		}
		if c.want == 401 && resp.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s %s: a 401 without WWW-Authenticate: Bearer", c.method, c.path)
		}
	}
}

// Without its secrets, the API refuses every token: an unset secret matches nothing, not "".
func TestUnsetSecretsMatchNothing(t *testing.T) {
	api := testAPI(t, nil)
	if resp := api.Do("GET", "/notes", "Authorization: Bearer "); resp.Code != 401 {
		t.Errorf("an empty token on a Worker without secrets: %d, want 401", resp.Code)
	}
	if resp := api.Do("GET", "/hello"); resp.Code != 200 {
		t.Errorf("hello is public: %d", resp.Code)
	}
}

// Access and an OIDC issuer, with the same scopes as the tokens: a person reads, a machine writes,
// an OIDC token does what its scope claim says. Each is refused (401) when it does not verify.
func TestAccessAndOIDC(t *testing.T) {
	issuer := authtest.New()
	defer issuer.Close()
	other := authtest.New()
	defer other.Close()
	settings := map[string]string{
		"WRITE_TOKEN": "w", AccessTeam: issuer.URL, AccessAUD: "app-aud", OIDCIssuer: issuer.URL, OIDCAudience: "https://notes.test",
	}
	api := testAPI(t, settings,
		Token{Secret: "WRITE_TOKEN", Scopes: []string{"read", "write"}},
		Access{People: []string{"read"}, Machines: []string{"read", "write"}},
		OIDC{},
	)
	access := func(token string) string { return AccessHeader + ": " + token }
	oidc := func(token string) string { return "Authorization: Bearer " + token }
	person, machine := issuer.Access("app-aud", "dev@example.com", ""), issuer.Access("app-aud", "", "ci.access")
	for _, c := range []struct {
		name, method, header string
		want                 int
		body                 string
	}{
		{"a person reads", "GET", access(person), 200, "dev@example.com"},
		{"a person may not write", "POST", access(person), 403, "needs scope write"},
		{"a machine writes", "POST", access(machine), 200, "service token ci.access"},
		{"an Access token for another application", "GET", access(issuer.Access("other-aud", "dev@example.com", "")), 401, "audience"},
		{"an Access token from another team", "GET", access(other.Access("app-aud", "dev@example.com", "")), 401, "refused"},
		{"an Access token that names no one", "GET", access(issuer.Token(map[string]any{"aud": "app-aud"})), 401, "names no one"},
		{"an OIDC token with write writes", "POST", oidc(issuer.OIDC("https://notes.test", "user-1", "openid write")), 200, "user-1"},
		{"an OIDC token with read may not write", "POST", oidc(issuer.OIDC("https://notes.test", "user-1", "read")), 403, "needs scope write"},
		{"an OIDC token for another API", "GET", oidc(issuer.OIDC("https://elsewhere.test", "user-1", "read")), 401, "audience"},
		{"an OIDC token from another issuer", "GET", oidc(other.OIDC("https://notes.test", "user-1", "read")), 401, "refused"},
		{"the bearer token beside them", "POST", "Authorization: Bearer w", 200, "WRITE_TOKEN"},
	} {
		resp := api.Do(c.method, "/notes", c.header)
		if resp.Code != c.want || !strings.Contains(resp.Body.String(), c.body) {
			t.Errorf("%s: %d %s, want %d with %q", c.name, resp.Code, resp.Body, c.want, c.body)
		}
	}
	// Unset, Access and OIDC trust no one: the same tokens are no credentials, or unknown ones.
	bare := testAPI(t, map[string]string{"WRITE_TOKEN": "w"}, Token{Secret: "WRITE_TOKEN", Scopes: []string{"read"}}, Access{People: []string{"read"}}, OIDC{})
	for _, header := range []string{access(person), oidc(issuer.OIDC("https://notes.test", "user-1", "read"))} {
		if resp := bare.Do("GET", "/notes", header); resp.Code != 401 {
			t.Errorf("without the settings, %s...: %d, want 401", header[:30], resp.Code)
		}
	}
}

// What must not verify, does not: each case is a token a forger or a careless issuer could make.
func TestTheJWTChecks(t *testing.T) {
	issuer := authtest.New()
	defer issuer.Close()
	k := &keyCache{}
	ctx := context.Background()
	verify := func(token string) error {
		_, err := k.oidc(ctx, issuer.URL, "https://notes.test", token)
		return err
	}
	now := time.Now().Unix()
	good := issuer.OIDC("https://notes.test", "u", "read")
	if err := verify(good); err != nil {
		t.Fatalf("a good token: %v", err)
	}
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	sign := func(alg jose.SignatureAlgorithm, key any, kid string, claims map[string]any) string {
		signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid))
		if err != nil {
			t.Fatal(err)
		}
		token, err := jwt.Signed(signer).Claims(claims).Serialize()
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	claims := map[string]any{"iss": issuer.URL, "aud": "https://notes.test", "sub": "u", "exp": now + 60}
	b64 := base64.RawURLEncoding.EncodeToString
	parts := strings.Split(good, ".")
	for name, token := range map[string]string{
		"expired":                   issuer.Token(map[string]any{"aud": "https://notes.test", "sub": "u", "exp": now - 120}),
		"not yet valid":             issuer.Token(map[string]any{"aud": "https://notes.test", "sub": "u", "nbf": now + 120}),
		"without exp":               issuer.Token(map[string]any{"aud": "https://notes.test", "sub": "u", "exp": nil}),
		"for another audience":      issuer.OIDC("https://elsewhere.test", "u", "read"),
		"from another issuer":       issuer.Token(map[string]any{"iss": "https://evil.test", "aud": "https://notes.test", "sub": "u"}),
		"alg none":                  b64([]byte(`{"alg":"none","kid":"test"}`)) + "." + parts[1] + ".",
		"HS256 with the public key": sign(jose.HS256, []byte("a shared secret anyone could guess"), "test", claims),
		"an unknown kid":            sign(jose.RS256, rsaKey, "nope", claims),
		"another key under its kid": sign(jose.RS256, rsaKey, "test", claims),
		"ES256 under an RSA kid":    sign(jose.ES256, ecKey, "test", claims),
		"a changed claim":           parts[0] + "." + b64([]byte(`{"iss":"`+issuer.URL+`","aud":"https://notes.test","sub":"admin","exp":`+strings.Repeat("9", 10)+`}`)) + "." + parts[2],
		"a changed signature":       parts[0] + "." + parts[1] + "." + b64([]byte("not the signature")),
		"not a JWT":                 "abc.def.ghi",
	} {
		if err := verify(token); err == nil {
			t.Errorf("%s: verified", name)
		} else {
			t.Logf("%s: %v", name, err)
		}
	}
}

// Keys are fetched once, and again for an unknown kid only after 30 s.
func TestKeysAreKept(t *testing.T) {
	issuer := authtest.New()
	defer issuer.Close()
	fetches := 0
	inner := issuer.Server.Config.Handler
	issuer.Server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cdn-cgi/access/certs" {
			fetches++
		}
		inner.ServeHTTP(w, r)
	})
	k := &keyCache{}
	for range 3 {
		if _, err := k.access(context.Background(), issuer.URL, "a", issuer.Access("a", "x@y", "")); err != nil {
			t.Fatal(err)
		}
	}
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: rsaKey}, (&jose.SignerOptions{}).WithHeader("kid", "new"))
	unknown, _ := jwt.Signed(signer).Claims(map[string]any{"iss": issuer.URL, "aud": "a", "email": "x@y", "exp": time.Now().Unix() + 60}).Serialize()
	k.access(context.Background(), issuer.URL, "a", unknown)
	if fetches != 1 {
		t.Errorf("the keys were fetched %d times, want 1", fetches)
	}
}

// The specs say what the server enforces: every scheme, the default, and each operation's scopes
// by each scheme.
func TestTheSpecSaysSo(t *testing.T) {
	api := testAPI(t, nil)
	spec, err := api.OpenAPI().MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"bearer":{"scheme":"bearer","type":"http"}`,
		`"accessClientId":{"description":`, `"env":"NOTES_TEST_ACCESS_CLIENT_ID"`,
		`"oidc":{"description":`, `"openIdConnectUrl":"/.well-known/openid-configuration"`,
		`"security":[{"bearer":["read"]},{"accessClientId":["read"],"accessClientSecret":[]},{"oidc":["read"]}]`,
		`"security":[{"bearer":["write"]},{"accessClientId":["write"],"accessClientSecret":[]},{"oidc":["write"]}]`,
		`"security":[]`,
	} {
		if !strings.Contains(string(spec), want) {
			t.Errorf("the spec has no %s", want)
		}
	}
}

func TestDiscovery(t *testing.T) {
	for issuer, want := range map[string]int{"": 404, "https://id.example.com": 302} {
		w := httptest.NewRecorder()
		Discovery(func(string) string { return issuer }).ServeHTTP(w, httptest.NewRequest("GET", "/.well-known/openid-configuration", nil))
		if w.Code != want || (want == 302 && w.Header().Get("Location") != issuer+"/.well-known/openid-configuration") {
			t.Errorf("issuer %q: %d %s", issuer, w.Code, w.Header().Get("Location"))
		}
	}
}
