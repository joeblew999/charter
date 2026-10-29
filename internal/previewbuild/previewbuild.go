// Package previewbuild hands a commit's build from CI to the Worker Preview deploy, so it is compiled once:
//
//  1. CI packs the build (Pack: the files plus a preview-build.json manifest of their sha256) and publishes it as
//     the asset <sha>.tar.gz of a pre-release (GitHub.Publish, keeping the newest N).
//  2. The preview build downloads it from the public release URL (Fetch, waiting until it appears) and unpacks it,
//     checking every file against the manifest and the manifest against the commit (Unpack).
//
// It also derives the preview's name from the branch (Slug), so CI knows the preview URL before the deploy, and
// posts it on the branch's pull request (GitHub.CommentPR). Local tooling (standard Go); never in the Worker.
package previewbuild

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// ManifestName is the manifest file inside every packed build.
const ManifestName = "preview-build.json"

// Manifest records which commit a build is and what it contains.
type Manifest struct {
	Commit string            `json:"commit"`
	Files  map[string]string `json:"files"` // slash path → sha256 hex
}

// Pack tars and gzips the files under the given paths (files or directories, relative to root) with a manifest for
// commit.
func Pack(root, commit string, paths ...string) ([]byte, error) {
	if commit == "" {
		return nil, errors.New("previewbuild: commit is required")
	}
	m := Manifest{Commit: commit, Files: map[string]string{}}
	var files []string
	for _, p := range paths {
		err := filepath.WalkDir(filepath.Join(root, p), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("previewbuild: %s: %w", p, err)
		}
	}
	slices.Sort(files)
	files = slices.Compact(files)
	if len(files) == 0 {
		return nil, errors.New("previewbuild: nothing to pack")
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		m.Files[f] = hex.EncodeToString(sum[:])
		if err := writeFile(tw, f, b, info.Mode().Perm()); err != nil {
			return nil, err
		}
	}
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeFile(tw, ManifestName, mb, 0o644); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeFile(tw *tar.Writer, name string, b []byte, mode os.FileMode) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: int64(mode), Size: int64(len(b)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := tw.Write(b)
	return err
}

// Unpack extracts a packed build into root and verifies it: the manifest names commit, every file is listed in
// it with a matching sha256, and every listed file is present. Paths can't leave root. It extracts into a
// temporary directory first and moves the files into root only once everything verified, so a rejected build
// leaves root untouched.
func Unpack(r io.Reader, root, commit string) (Manifest, error) {
	tmp, err := os.MkdirTemp(root, ".previewbuild-")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(tmp)
	m, err := unpack(r, tmp, commit)
	if err != nil {
		return m, err
	}
	for f := range m.Files {
		dst := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return m, err
		}
		if err := os.Rename(filepath.Join(tmp, filepath.FromSlash(f)), dst); err != nil {
			return m, err
		}
	}
	return m, nil
}

func unpack(r io.Reader, root, commit string) (Manifest, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return Manifest{}, fmt.Errorf("previewbuild: not a gzip build: %w", err)
	}
	tr := tar.NewReader(gz)
	got := map[string]string{}
	var m *Manifest
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Manifest{}, fmt.Errorf("previewbuild: %w", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.ToSlash(filepath.Clean(h.Name))
		if strings.HasPrefix(name, "../") || name == ".." || filepath.IsAbs(h.Name) {
			return Manifest{}, fmt.Errorf("previewbuild: unsafe path %q", h.Name)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return Manifest{}, err
		}
		if name == ManifestName {
			m = new(Manifest)
			if err := json.Unmarshal(b, m); err != nil {
				return Manifest{}, fmt.Errorf("previewbuild: manifest: %w", err)
			}
			continue
		}
		sum := sha256.Sum256(b)
		got[name] = hex.EncodeToString(sum[:])
		dst := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return Manifest{}, err
		}
		if err := os.WriteFile(dst, b, os.FileMode(h.Mode).Perm()|0o600); err != nil {
			return Manifest{}, err
		}
	}
	if m == nil {
		return Manifest{}, errors.New("previewbuild: no " + ManifestName)
	}
	if m.Commit != commit {
		return *m, fmt.Errorf("previewbuild: build is for commit %s, want %s", m.Commit, commit)
	}
	for f, want := range m.Files {
		if got[f] != want {
			return *m, fmt.Errorf("previewbuild: %s: checksum mismatch or missing", f)
		}
	}
	for f := range got {
		if _, ok := m.Files[f]; !ok {
			return *m, fmt.Errorf("previewbuild: %s is not in the manifest", f)
		}
	}
	return *m, nil
}

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug is the preview name for a branch of the Worker app: lowercase letters, digits and single hyphens, short
// enough that "<slug>-<app>" fits a DNS label (63), with a hash suffix when it had to be shortened so two long
// branches don't collide. The preview answers at https://<slug>-<app>.<subdomain>.workers.dev.
func Slug(branch, app string) string {
	s := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(branch), "-"), "-")
	if s == "" {
		s = "branch"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "b-" + s
	}
	limit := 63 - len(app) - 1
	if len(s) > limit {
		sum := sha256.Sum256([]byte(branch))
		h := hex.EncodeToString(sum[:])[:6]
		s = strings.TrimRight(s[:max(1, limit-7)], "-") + "-" + h
	}
	return s
}

// URL is a preview's workers.dev URL.
func URL(slug, app, subdomain string) string {
	return "https://" + slug + "-" + app + "." + subdomain + ".workers.dev"
}

// GitHub talks to one repository's REST API. Public downloads need no token; publishing and commenting do.
type GitHub struct {
	Repo      string       // owner/name
	Token     string       // e.g. CI's GITHUB_TOKEN
	API       string       // default https://api.github.com
	Uploads   string       // default https://uploads.github.com
	Downloads string       // default https://github.com
	HTTP      *http.Client // default http.DefaultClient
	Logf      func(format string, args ...any)
	// Title and About name and describe a pre-release Publish creates (default: the preview builds').
	Title, About string
}

func (g *GitHub) logf(format string, args ...any) {
	if g.Logf != nil {
		g.Logf(format, args...)
	}
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (g *GitHub) client() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return http.DefaultClient
}

// do sends a request to the API (or a full URL) and decodes a JSON answer into out. A 404 returns errNotFound.
func (g *GitHub) do(ctx context.Context, method, pathOrURL, contentType string, body []byte, out any) error {
	u := pathOrURL
	if strings.HasPrefix(u, "/") {
		u = or(g.API, "https://api.github.com") + u
	}
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := g.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d: %.300s", method, u, resp.StatusCode, raw)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

var errNotFound = errors.New("not found")

type release struct {
	ID     int64   `json:"id"`
	Assets []asset `json:"assets"`
}

type asset struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Publish uploads data as the asset name of the pre-release tag (created if missing), replacing an asset of the
// same name, then deletes all but the newest keep assets.
func (g *GitHub) Publish(ctx context.Context, tag, name string, data []byte, keep int) error {
	var rel release
	err := g.do(ctx, "GET", "/repos/"+g.Repo+"/releases/tags/"+url.PathEscape(tag), "", nil, &rel)
	if errors.Is(err, errNotFound) {
		body, _ := json.Marshal(map[string]any{"tag_name": tag, "name": or(g.Title, "Preview builds"), "prerelease": true,
			"body": or(g.About, "Build output per commit for Cloudflare Worker Previews (charter deploy). Not a release.")})
		err = g.do(ctx, "POST", "/repos/"+g.Repo+"/releases", "application/json", body, &rel)
		if err == nil {
			g.logf("✓ release %s created", tag)
		}
	}
	if err != nil {
		return fmt.Errorf("previewbuild: release %s: %w", tag, err)
	}
	for _, a := range rel.Assets {
		if a.Name == name {
			if err := g.do(ctx, "DELETE", fmt.Sprintf("/repos/%s/releases/assets/%d", g.Repo, a.ID), "", nil, nil); err != nil {
				return fmt.Errorf("previewbuild: replace %s: %w", name, err)
			}
		}
	}
	up := fmt.Sprintf("%s/repos/%s/releases/%d/assets?name=%s", or(g.Uploads, "https://uploads.github.com"), g.Repo, rel.ID, url.QueryEscape(name))
	var added asset
	if err := g.do(ctx, "POST", up, "application/gzip", data, &added); err != nil {
		return fmt.Errorf("previewbuild: upload %s: %w", name, err)
	}
	g.logf("✓ published %s/%s (%d bytes)", tag, name, len(data))

	assets := append([]asset{added}, slices.DeleteFunc(rel.Assets, func(a asset) bool { return a.Name == name })...)
	sort.SliceStable(assets, func(i, j int) bool { return assets[i].CreatedAt.After(assets[j].CreatedAt) })
	if keep > 0 && len(assets) > keep {
		for _, a := range assets[keep:] {
			if a.ID == added.ID {
				continue
			}
			if err := g.do(ctx, "DELETE", fmt.Sprintf("/repos/%s/releases/assets/%d", g.Repo, a.ID), "", nil, nil); err != nil {
				return fmt.Errorf("previewbuild: prune %s: %w", a.Name, err)
			}
			g.logf("· pruned %s", a.Name)
		}
	}
	return nil
}

// Fetch downloads the public asset name of release tag, retrying every interval until it exists or wait runs
// out. It reports false (and no error) when it never appeared.
func (g *GitHub) Fetch(ctx context.Context, tag, name string, wait, interval time.Duration) ([]byte, bool, error) {
	u := fmt.Sprintf("%s/%s/releases/download/%s/%s", or(g.Downloads, "https://github.com"), g.Repo, url.PathEscape(tag), url.PathEscape(name))
	deadline := time.Now().Add(wait)
	for {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, false, err
		}
		resp, err := g.client().Do(req)
		if err == nil {
			b, rerr := io.ReadAll(resp.Body)
			resp.Body.Close()
			switch {
			case resp.StatusCode == http.StatusOK && rerr == nil:
				return b, true, nil
			case resp.StatusCode != http.StatusNotFound:
				g.logf("· %s: HTTP %d, retrying", name, resp.StatusCode)
			}
		} else {
			g.logf("· %s: %v, retrying", name, err)
		}
		if !time.Now().Add(interval).Before(deadline) {
			return nil, false, nil
		}
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// CommentPR writes body on the open pull request whose head is branch: it edits the comment that starts with
// marker, or adds one. It reports false (and no error) when the branch has no open pull request.
func (g *GitHub) CommentPR(ctx context.Context, branch, marker, body string) (bool, error) {
	owner, _, _ := strings.Cut(g.Repo, "/")
	var prs []struct {
		Number int `json:"number"`
	}
	q := url.Values{"state": {"open"}, "head": {owner + ":" + branch}}
	if err := g.do(ctx, "GET", "/repos/"+g.Repo+"/pulls?"+q.Encode(), "", nil, &prs); err != nil {
		return false, fmt.Errorf("previewbuild: pull requests for %s: %w", branch, err)
	}
	if len(prs) == 0 {
		return false, nil
	}
	n := prs[0].Number
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := g.do(ctx, "GET", fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100", g.Repo, n), "", nil, &comments); err != nil {
		return false, fmt.Errorf("previewbuild: comments on #%d: %w", n, err)
	}
	text, _ := json.Marshal(map[string]string{"body": marker + "\n" + body})
	for _, c := range comments {
		if strings.HasPrefix(c.Body, marker) {
			if err := g.do(ctx, "PATCH", fmt.Sprintf("/repos/%s/issues/comments/%d", g.Repo, c.ID), "application/json", text, nil); err != nil {
				return false, fmt.Errorf("previewbuild: edit comment on #%d: %w", n, err)
			}
			g.logf("✓ PR #%d comment updated", n)
			return true, nil
		}
	}
	if err := g.do(ctx, "POST", fmt.Sprintf("/repos/%s/issues/%d/comments", g.Repo, n), "application/json", text, nil); err != nil {
		return false, fmt.Errorf("previewbuild: comment on #%d: %w", n, err)
	}
	g.logf("✓ PR #%d comment added", n)
	return true, nil
}
