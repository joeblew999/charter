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
// which charter-ts wrote: the same groups, generators and schemes, line for line, but for the words that name the
// command that writes them.
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
			if want := charterTSWording.Replace(string(committed)); got != want {
				t.Errorf("settings differ from the committed ones\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// charterTSWording is what charter-ts wrote where charter writes its own command.
var charterTSWording = strings.NewReplacer(
	"# Written by `charter-ts spec` from fern/openapi.json and charter.config.ts (sdks): don't edit, change\n# those and run npm run spec. Fern's generators are charter-ts's, at the versions it pins.",
	"# Written by `charter spec` from fern/openapi.json and charter.toml ([sdks]): don't edit, change\n# those and run charter spec. Fern's generators are charter's, at the versions it pins.",
	"# The SDKs. Generate one: npm run sdk -- <group>   (into sdk/out/<group>)",
	"# The SDKs. Generate one: charter sdk <group>   (into sdk/out/<group>)",
	"Go: the module npm run sdk -- --publish commits in sdk/go",
	"Go: the module charter sdk -publish commits in sdk/go",
)

// TestSpecsHashAsCommitted: the hash of fern/ is the one charter-ts recorded beside the Go SDK notes-go commits (made
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

// TestSamePinsAsChartersTS: until the TypeScript projects move to charter too (#121), charter-ts pins Fern's generators
// and the toolchains of a proof for them: the same versions as these, so both kinds of project get the same SDKs.
func TestSamePinsAsChartersTS(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "charter-ts", "src", "sdk.ts"))
	if err != nil {
		t.Skipf("no charter-ts beside it: %v", err)
	}
	ts := string(b)
	for _, g := range generators {
		if want := `name: "` + g.name + `", version: "` + g.version + `"`; !strings.Contains(ts, want) {
			t.Errorf("charter-ts/src/sdk.ts has no %s", want)
		}
	}
	for _, want := range []string{`rustVersion = "` + rustVersion + `"`, `uvVersion = "` + uvVersion + `", pythonVersion = "` + pythonVersion + `"`,
		`pythonPackages = ["` + strings.Join(pythonPackages, `", "`) + `"]`} {
		if !strings.Contains(ts, want) {
			t.Errorf("charter-ts/src/sdk.ts has no %s", want)
		}
	}
}
