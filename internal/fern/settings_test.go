package fern

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/joeblew999/charter/project"
)

// TestSettingsAsCommitted: for every example, the settings written from its spec and SDKs are the ones it commits,
// line for line.
func TestSettingsAsCommitted(t *testing.T) {
	notes := project.SDKs{Name: "Notes", More: []string{"python", "cli"}}
	conformance := project.SDKs{Name: "Conformance", More: []string{"python", "cli"}, Audiences: []string{"public"}}
	api := project.SDKs{Name: "Api"}
	for example, sdks := range map[string]project.SDKs{
		"notes-go": notes, "notes-ts": notes, "conformance-go": conformance, "conformance-ts": conformance,
		"start-go": api, "start-ts": api, "start-htmx": api, "start-datastar": api,
	} {
		t.Run(example, func(t *testing.T) {
			dir := filepath.Join("..", "..", "examples", example)
			spec, err := os.ReadFile(filepath.Join(dir, "fern", "openapi.json"))
			if err != nil {
				t.Fatal(err)
			}
			committed, err := os.ReadFile(filepath.Join(dir, "fern", "generators.yml"))
			if err != nil {
				t.Fatal(err)
			}
			module := "charter-" + example
			if gomod, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
				module = regexp.MustCompile(`(?m)^module\s+(\S+)`).FindStringSubmatch(string(gomod))[1]
			}
			_, err = os.Stat(filepath.Join(dir, "fern", "overlays.yml"))
			got, err := Settings(spec, sdks, module+"/sdk/go", err == nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := string(committed); got != want {
				t.Errorf("settings differ from the committed ones\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestSpecsHashAsCommitted: the hash of fern/ is the one recorded beside the Go SDK notes-go commits (made
// from these specs), so a Go SDK made before stays fresh; and the groups are its settings', in their order.
func TestSpecsHashAsCommitted(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "notes-go")
	made, err := os.ReadFile(filepath.Join(dir, "sdk", "go", ".made-from"))
	if err != nil {
		t.Fatal(err)
	}
	if hash, err := SpecsHash(dir); err != nil || hash != strings.TrimSpace(string(made)) {
		t.Errorf("SpecsHash = %q, %v; sdk/go/.made-from says %q", hash, err, made)
	}
	if err := GoSDKFresh(dir); err != nil {
		t.Error(err)
	}
	groups, err := Groups(dir)
	if want := []string{"go", "typescript", "typescript-dist", "python", "cli"}; err != nil || !slices.Equal(groups, want) {
		t.Errorf("Groups = %v, %v; want %v", groups, err, want)
	}
}

// TestProxyCA: a proxy on this machine with a CA it can find is one; a remote proxy, or none, is not.
func TestProxyCA(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	has := func(p string) bool { return p == cloudCA || p == "/ca.pem" }
	if ca := ProxyCA(env(map[string]string{"HTTPS_PROXY": "http://127.0.0.1:8080", "NODE_EXTRA_CA_CERTS": "/ca.pem"}), has); ca != "/ca.pem" {
		t.Errorf("local proxy, CA named: %q", ca)
	}
	if ca := ProxyCA(env(map[string]string{"HTTPS_PROXY": "http://localhost:3128"}), has); ca != cloudCA {
		t.Errorf("local proxy, the cloud session's CA: %q", ca)
	}
	if ca := ProxyCA(env(map[string]string{"HTTPS_PROXY": "http://proxy.corp:3128"}), has); ca != "" {
		t.Errorf("remote proxy: %q", ca)
	}
	if ca := ProxyCA(env(nil), has); ca != "" {
		t.Errorf("no proxy: %q", ca)
	}
}
