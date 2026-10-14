package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// The JWT checks, made by go-jose: the algorithms it accepts (never none, never HMAC), and the
// clock skew allowed on exp, nbf and iat.
var algorithms = []jose.SignatureAlgorithm{jose.RS256, jose.ES256, jose.EdDSA}

const leeway = time.Minute

// How long fetched keys and discovery documents are kept, and the least time between two fetches of
// one key set: a token with a key id the set lacks makes one new fetch, at most this often.
const (
	keysKept     = time.Hour
	refetchAfter = 30 * time.Second
)

// The errors a JWT can be refused with.
var (
	errMalformed = errors.New("not a JWT this API accepts (RS256, ES256 or EdDSA, signed once)")
	errKey       = errors.New("signed with a key the issuer does not publish")
	errSignature = errors.New("the signature does not verify")
	errNoOne     = errors.New("the token names no one")
)

// keyCache verifies tokens against issuers' published keys, which it keeps between requests (one
// cache per Middleware, so per Worker isolate).
type keyCache struct {
	mu         sync.Mutex
	sets       map[string]*keySet    // by JWKS URL
	discovered map[string]*discovery // by issuer
}

type keySet struct {
	keys    jose.JSONWebKeySet
	fetched time.Time
}

type discovery struct {
	Issuer  string `json:"issuer"`
	JWKS    string `json:"jwks_uri"`
	fetched time.Time
}

// access verifies a JWT that Cloudflare Access sent: issued by the team, for the application aud.
// A person has email; a service token has common_name, its Client ID.
func (k *keyCache) access(ctx context.Context, team, aud, token string) (Caller, error) {
	team = strings.TrimSuffix(team, "/")
	if !strings.HasPrefix(team, "http://") { // http only for a test issuer on localhost
		team = "https://" + strings.TrimPrefix(team, "https://")
	}
	var claims struct {
		Email      string `json:"email"`
		CommonName string `json:"common_name"`
	}
	std, err := k.verify(ctx, token, team, team+"/cdn-cgi/access/certs", aud, &claims)
	if err != nil {
		return Caller{}, fmt.Errorf("the Access token: %w", err)
	}
	c := Caller{Scheme: "access", Email: claims.Email, Subject: std.Subject}
	if c.Email == "" {
		c.Machine = claims.CommonName
	}
	if c.Email == "" && c.Machine == "" {
		return Caller{}, fmt.Errorf("the Access token: %w", errNoOne)
	}
	return c, nil
}

// oidc verifies an access token from the issuer, for audience: the issuer's keys are where its
// discovery document says.
func (k *keyCache) oidc(ctx context.Context, issuer, audience, token string) (Caller, error) {
	d, err := k.discover(ctx, issuer)
	if err != nil {
		return Caller{}, err
	}
	var claims struct {
		Email string   `json:"email"`
		Scope string   `json:"scope"`
		Scp   []string `json:"scp"`
	}
	std, err := k.verify(ctx, token, d.Issuer, d.JWKS, audience, &claims)
	if err != nil {
		return Caller{}, fmt.Errorf("the OpenID Connect token: %w", err)
	}
	if std.Subject == "" {
		return Caller{}, fmt.Errorf("the OpenID Connect token: %w", errNoOne)
	}
	scopes := strings.Fields(claims.Scope)
	if len(scopes) == 0 {
		scopes = claims.Scp
	}
	return Caller{Scheme: OIDCName, Email: claims.Email, Subject: std.Subject, Scopes: scopes}, nil
}

// verify checks token's signature against the issuer's keys at jwks, then its claims: iss, aud,
// exp (which it must have), nbf and iat. extra receives the claims the caller wants besides.
func (k *keyCache) verify(ctx context.Context, token, issuer, jwks, audience string, extra any) (jwt.Claims, error) {
	parsed, err := jwt.ParseSigned(token, algorithms)
	if err != nil || len(parsed.Headers) != 1 {
		return jwt.Claims{}, errMalformed
	}
	key, err := k.key(ctx, jwks, parsed.Headers[0].KeyID)
	if err != nil {
		return jwt.Claims{}, err
	}
	var std jwt.Claims
	if err := parsed.Claims(key, &std, extra); err != nil {
		return jwt.Claims{}, errSignature
	}
	if std.Expiry == nil {
		return jwt.Claims{}, jwt.ErrExpired
	}
	if err := std.ValidateWithLeeway(jwt.Expected{Issuer: issuer, AnyAudience: jwt.Audience{audience}, Time: time.Now()}, leeway); err != nil {
		return jwt.Claims{}, err
	}
	return std, nil
}

// key is the public key kid of the set at url: from the kept set, or from a new fetch when the set
// is old or lacks it (at most every refetchAfter).
func (k *keyCache) key(ctx context.Context, url, kid string) (any, error) {
	k.mu.Lock()
	set := k.sets[url]
	k.mu.Unlock()
	now := time.Now()
	if set != nil && now.Sub(set.fetched) < keysKept {
		if key, ok := find(set.keys, kid); ok {
			return key, nil
		}
		if now.Sub(set.fetched) < refetchAfter {
			return nil, errKey
		}
	}
	var keys jose.JSONWebKeySet
	if err := fetchJSON(ctx, url, &keys); err != nil {
		return nil, err
	}
	k.mu.Lock()
	if k.sets == nil {
		k.sets = map[string]*keySet{}
	}
	k.sets[url] = &keySet{keys: keys, fetched: now}
	k.mu.Unlock()
	if key, ok := find(keys, kid); ok {
		return key, nil
	}
	return nil, errKey
}

// find is the signing key kid of a set; with no kid, the set's only signing key.
func find(set jose.JSONWebKeySet, kid string) (any, bool) {
	var found []jose.JSONWebKey
	for _, key := range set.Keys {
		if key.Use != "enc" && key.IsPublic() && (kid == "" || key.KeyID == kid) {
			found = append(found, key)
		}
	}
	if len(found) != 1 {
		return nil, false
	}
	return found[0].Key, true
}

// discover is the issuer's discovery document, kept for keysKept. Its issuer must be the one asked
// for (OpenID Connect Discovery 1.0, 4.3).
func (k *keyCache) discover(ctx context.Context, issuer string) (*discovery, error) {
	k.mu.Lock()
	d := k.discovered[issuer]
	k.mu.Unlock()
	if d != nil && time.Since(d.fetched) < keysKept {
		return d, nil
	}
	d = &discovery{}
	if err := fetchJSON(ctx, strings.TrimSuffix(issuer, "/")+"/.well-known/openid-configuration", d); err != nil {
		return nil, err
	}
	if d.Issuer != issuer || d.JWKS == "" {
		return nil, fmt.Errorf("the discovery document of %s names the issuer %q and the keys %q", issuer, d.Issuer, d.JWKS)
	}
	d.fetched = time.Now()
	k.mu.Lock()
	if k.discovered == nil {
		k.discovered = map[string]*discovery{}
	}
	k.discovered[issuer] = d
	k.mu.Unlock()
	return d, nil
}

func fetchJSON(ctx context.Context, url string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client().Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil || res.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching %s: HTTP %d %v", url, res.StatusCode, err)
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("reading %s: %w", url, err)
	}
	return nil
}
