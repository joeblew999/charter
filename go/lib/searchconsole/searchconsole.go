// Package searchconsole talks to the Google Search Console API with a service account, standard library only: submit
// and read sitemaps, and inspect how Google indexed a URL. Test live URL and Request indexing have no API (Search
// Console UI only).
//
// APIs: https://developers.google.com/webmaster-tools/v1/sitemaps (webmasters/v3 sitemaps.submit, sitemaps.get) and
// https://developers.google.com/webmaster-tools/v1/urlInspection.index/inspect (searchconsole/v1). Service account
// auth is the OAuth 2.0 JWT bearer grant (RFC 7523): https://developers.google.com/identity/protocols/oauth2/service-account.
// The service account must be added as a user of the property in Search Console (Settings → Users and permissions).
package searchconsole

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Scope is the Search Console scope with write access (submitting sitemaps needs it).
const Scope = "https://www.googleapis.com/auth/webmasters"

// Key is a service account JSON key (the fields this package uses).
type Key struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// ParseKey reads a service account JSON key.
func ParseKey(b []byte) (Key, error) {
	var k Key
	if err := json.Unmarshal(b, &k); err != nil {
		return k, fmt.Errorf("searchconsole: key: %w", err)
	}
	if k.ClientEmail == "" || k.PrivateKey == "" {
		return k, errors.New("searchconsole: key lacks client_email or private_key")
	}
	if k.TokenURI == "" {
		k.TokenURI = "https://oauth2.googleapis.com/token"
	}
	return k, nil
}

// Token exchanges a signed JWT assertion for an access token.
func (k Key) Token(ctx context.Context, client *http.Client, scope string) (string, error) {
	block, _ := pem.Decode([]byte(k.PrivateKey))
	if block == nil {
		return "", errors.New("searchconsole: private_key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("searchconsole: private_key: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", errors.New("searchconsole: private_key is not RSA")
	}
	now := time.Now()
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	unsigned := enc(map[string]string{"alg": "RS256", "typ": "JWT"}) + "." + enc(map[string]any{
		"iss": k.ClientEmail, "scope": scope, "aud": k.TokenURI, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)}}
	req, err := http.NewRequestWithContext(ctx, "POST", k.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := do(client, req, &out); err != nil {
		return "", fmt.Errorf("searchconsole: token: %w", err)
	}
	if out.AccessToken == "" {
		return "", errors.New("searchconsole: token response has no access_token")
	}
	return out.AccessToken, nil
}

// Client calls the Search Console API for one property.
type Client struct {
	HTTP  *http.Client
	Token string
	// Site is the property: "sc-domain:example.com" for a Domain property, or a URL-prefix property's URL.
	Site string
	// Base URLs; tests point them at a fake server.
	WebmastersBase    string // default https://www.googleapis.com/webmasters/v3
	SearchConsoleBase string // default https://searchconsole.googleapis.com/v1
}

func (c *Client) webmasters() string {
	if c.WebmastersBase != "" {
		return c.WebmastersBase
	}
	return "https://www.googleapis.com/webmasters/v3"
}

func (c *Client) searchConsole() string {
	if c.SearchConsoleBase != "" {
		return c.SearchConsoleBase
	}
	return "https://searchconsole.googleapis.com/v1"
}

func (c *Client) call(ctx context.Context, method, u string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return do(c.HTTP, req, out)
}

func do(client *http.Client, req *http.Request, out any) error {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			Description string `json:"error_description"`
		}
		json.Unmarshal(b, &e)
		msg := e.Error.Message + e.Description
		if msg == "" {
			msg = strings.TrimSpace(string(b))
		}
		return fmt.Errorf("%s %s: %s: %s", req.Method, req.URL.Path, res.Status, msg)
	}
	if out != nil && len(b) > 0 {
		return json.Unmarshal(b, out)
	}
	return nil
}

// Site is a property the service account can use.
type Site struct {
	SiteURL         string `json:"siteUrl"`
	PermissionLevel string `json:"permissionLevel"`
}

// Sites lists the properties the credentials can access.
func (c *Client) Sites(ctx context.Context) ([]Site, error) {
	var out struct {
		SiteEntry []Site `json:"siteEntry"`
	}
	err := c.call(ctx, "GET", c.webmasters()+"/sites", nil, &out)
	return out.SiteEntry, err
}

// SubmitSitemap submits (or resubmits) a sitemap URL.
func (c *Client) SubmitSitemap(ctx context.Context, feed string) error {
	return c.call(ctx, "PUT", c.webmasters()+"/sites/"+url.PathEscape(c.Site)+"/sitemaps/"+url.PathEscape(feed), nil, nil)
}

// Sitemap is a submitted sitemap's status.
type Sitemap struct {
	Path           string `json:"path"`
	LastSubmitted  string `json:"lastSubmitted"`
	LastDownloaded string `json:"lastDownloaded"`
	IsPending      bool   `json:"isPending"`
	Warnings       string `json:"warnings"`
	Errors         string `json:"errors"`
	Contents       []struct {
		Type      string `json:"type"`
		Submitted string `json:"submitted"`
		Indexed   string `json:"indexed"`
	} `json:"contents"`
}

// GetSitemap reads a submitted sitemap's status.
func (c *Client) GetSitemap(ctx context.Context, feed string) (Sitemap, error) {
	var s Sitemap
	err := c.call(ctx, "GET", c.webmasters()+"/sites/"+url.PathEscape(c.Site)+"/sitemaps/"+url.PathEscape(feed), nil, &s)
	return s, err
}

// Inspection is the index status of one URL (urlInspection.index.inspect's indexStatusResult, the fields we report).
type Inspection struct {
	Verdict         string `json:"verdict"`       // PASS, PARTIAL, FAIL, NEUTRAL
	CoverageState   string `json:"coverageState"` // e.g. "Submitted and indexed", "URL is unknown to Google"
	RobotsTxtState  string `json:"robotsTxtState"`
	IndexingState   string `json:"indexingState"`
	PageFetchState  string `json:"pageFetchState"`
	LastCrawlTime   string `json:"lastCrawlTime"`
	GoogleCanonical string `json:"googleCanonical"`
	UserCanonical   string `json:"userCanonical"`
	CrawledAs       string `json:"crawledAs"`
	// Link is Google's own Search Console page for this inspection (inspectionResultLink): Test live URL, Request indexing.
	Link string `json:"-"`
}

// Inspect reports how Google indexed u (Google's index, not a live fetch). languageCode localizes the messages.
func (c *Client) Inspect(ctx context.Context, u, languageCode string) (Inspection, error) {
	var out struct {
		InspectionResult struct {
			InspectionResultLink string     `json:"inspectionResultLink"`
			IndexStatusResult    Inspection `json:"indexStatusResult"`
		} `json:"inspectionResult"`
	}
	body := map[string]string{"inspectionUrl": u, "siteUrl": c.Site, "languageCode": languageCode}
	err := c.call(ctx, "POST", c.searchConsole()+"/urlInspection/index:inspect", body, &out)
	in := out.InspectionResult.IndexStatusResult
	in.Link = out.InspectionResult.InspectionResultLink
	return in, err
}
