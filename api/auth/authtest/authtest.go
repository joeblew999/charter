// Package authtest is an issuer for tests: it serves its keys over HTTP as Cloudflare Access does
// (/cdn-cgi/access/certs) and as an OpenID Connect issuer does (/.well-known/openid-configuration,
// /jwks), and signs tokens with them (go-jose, RS256). Point ACCESS_TEAM_DOMAIN or OIDC_ISSUER at
// its URL in a test, never in a deployment.
package authtest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Issuer is a running test issuer.
type Issuer struct {
	URL    string // its origin: the Access team domain, and the OIDC issuer
	Server *httptest.Server
	key    *rsa.PrivateKey
}

// New starts an issuer; Close it after.
func New() *Issuer {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	i := &Issuer{key: key}
	i.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/openid-configuration" {
			json.NewEncoder(w).Encode(map[string]string{"issuer": i.URL, "jwks_uri": i.URL + "/jwks"})
			return
		}
		w.Write(i.JWKS())
	}))
	i.URL = i.Server.URL
	return i
}

func (i *Issuer) Close() { i.Server.Close() }

// JWKS is the issuer's key set document.
func (i *Issuer) JWKS() []byte {
	b, _ := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &i.key.PublicKey, KeyID: "test", Algorithm: string(jose.RS256), Use: "sig"}}})
	return b
}

// Access is what Access would sign for a person (email) or a service token (clientID), for the
// application aud.
func (i *Issuer) Access(aud, email, clientID string) string {
	claims := map[string]any{"aud": []string{aud}, "type": "app"}
	if email != "" {
		claims["email"], claims["sub"] = email, "user-"+email
	} else {
		claims["common_name"], claims["sub"] = clientID, ""
	}
	return i.Token(claims)
}

// OIDC is an access token for audience with these scopes (space-separated).
func (i *Issuer) OIDC(audience, subject, scope string) string {
	return i.Token(map[string]any{"aud": audience, "sub": subject, "scope": scope})
}

// Token signs claims, adding iss, iat and a 5-minute exp unless given; a claim given as nil is left out.
func (i *Issuer) Token(claims map[string]any) string {
	now := time.Now().Unix()
	for name, value := range map[string]any{"iss": i.URL, "iat": now, "exp": now + 300} {
		if _, given := claims[name]; !given {
			claims[name] = value
		}
	}
	for name, value := range claims {
		if value == nil {
			delete(claims, name)
		}
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: i.key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test"))
	if err != nil {
		panic(err)
	}
	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		panic(err)
	}
	return token
}
