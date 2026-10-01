package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func init() {
	commands["upstream"] = command{"", "the upstream issues our code works around (every `Upstream:` tag) and their state", upstream}
	commands["doctor"] = command{"", "check what the tasks need: npm installs, Docker, Go, TinyGo, Rust, leftover containers", doctor}
	commands["cloudflare-spec"] = command{"[-products d1,kv] [-release <sha>]",
		"HEAVY (26 MB): slice Cloudflare products out of Forge's spec into sdk/fern/apis/cloudflare", cloudflareSpec}
}

// A tag is a comment line such as:  Upstream: owner/repo#123 (when fixed: what to do then)
var upstreamTag = regexp.MustCompile(`^([^:]+:[0-9]+):.*Upstream: ([\w.-]+/[\w.-]+)#([0-9]+) *(.*)$`)

// upstream finds the tags and asks GitHub for each issue's state. CLOSED means that workaround can
// go (the table in docs/upstream.md says what to do).
func upstream([]string) error {
	out, err := output(".", "git", "grep", "-n", "-E", `Upstream: [A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#[0-9]+`, "--", ":!*.md", ":!mise.toml", ":!dev/")
	if err != nil {
		return errors.New("no Upstream: tags found")
	}
	places := map[string][]string{}
	for _, line := range strings.Split(out, "\n") {
		if m := upstreamTag.FindStringSubmatch(line); m != nil {
			issue := m[2] + "#" + m[3]
			places[issue] = append(places[issue], fmt.Sprintf("        %s  %s", m[1], m[4]))
		}
	}
	issues := make([]string, 0, len(places))
	for issue := range places {
		issues = append(issues, issue)
	}
	sort.Strings(issues)
	closed := 0
	for _, issue := range issues {
		repo, number, _ := strings.Cut(issue, "#")
		state, title := "UNREACHABLE", ""
		if info, err := output(".", "gh", "api", "repos/"+repo+"/issues/"+number, "--jq", `.state + "\t" + .title`); err == nil {
			state, title, _ = strings.Cut(info, "\t")
			state = strings.ToUpper(state)
		}
		if state == "CLOSED" {
			closed++
		}
		fmt.Printf("%s  %s  %s\n%s\n", state, issue, title, strings.Join(places[issue], "\n"))
	}
	fmt.Printf("\n%d upstream issues, %d closed", len(issues), closed)
	if closed > 0 {
		fmt.Print(": remove those workarounds (docs/upstream.md)")
	}
	fmt.Println()
	return nil
}

func doctor([]string) error {
	failed := false
	ok := func(format string, a ...any) { fmt.Printf("  ok    "+format+"\n", a...) }
	warn := func(format string, a ...any) { fmt.Printf("  WARN  "+format+"\n", a...) }
	bad := func(format string, a ...any) { fmt.Printf("  FAIL  "+format+"\n", a...); failed = true }
	for _, dir := range []string{"api", "api-go", "sdk", "sdk/harness"} {
		if exists(filepath.Join(dir, "node_modules")) {
			ok("npm packages in %s", dir)
		} else {
			bad("no npm packages in %s: mise run setup", dir)
		}
	}
	if docker() == nil {
		ok("docker running (Fern generates in containers)")
	} else {
		bad("docker not running: start Docker")
	}
	for _, tool := range []struct{ name, arg, why string }{
		{"go", "version", "api-go, dev, sdk:check on Go SDKs"},
		{"tinygo", "version", "api-go:build"},
		{"wasm-opt", "--version", "tinygo runs it on every Wasm build"},
		{"cargo", "--version", "the Fern CLI builds"},
		{"gh", "--version", "upstream:status"},
	} {
		if out, err := exec.Command(tool.name, tool.arg).Output(); err == nil {
			ok("%s (%s)", strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0], tool.why)
		} else {
			warn("no %s: mise install (%s)", tool.name, tool.why)
		}
	}
	if os.Getenv("FERN_TOKEN") != "" {
		ok("FERN_TOKEN set")
	} else {
		warn("FERN_TOKEN not set: Fern documents local generation as an Enterprise feature needing it (it ran without one on 2026-10-01)")
	}
	if ids, _ := output(".", "docker", "ps", "-q", "--filter", wiremock); ids == "" {
		ok("no leftover WireMock containers")
	} else {
		warn("%d WireMock container(s) running: mise run sdk:clean", len(strings.Fields(ids)))
	}
	if failed {
		return errors.New("not ready")
	}
	return sdkList(nil)
}

// cloudflareSpec adds Cloudflare products as an API for Fern, sliced from Forge's spec of the whole
// Cloudflare API. Then: mise run sdk:gen cloudflare go (or typescript, python).
func cloudflareSpec(args []string) error {
	products, release := "d1,kv", "6b0fb3cd63aca815f1667a8fa908114886867dc6"
	flags("cloudflare-spec", args, func(f *flag.FlagSet) {
		f.StringVar(&products, "products", products, "comma-separated: workers, d1, kv, r2, queues, workflows, ...")
		f.StringVar(&release, "release", release, "Forge openapi release")
	})
	full, api := filepath.Join(".forge", release+".json"), "sdk/fern/apis/cloudflare"
	if !exists(full) {
		res, err := http.Get("https://github.com/cloudflare/forge/releases/download/openapi@" + release + "/openapi.forge.json")
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("Forge release %s: HTTP %d", release, res.StatusCode)
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(".forge", 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			return err
		}
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	var spec map[string]json.RawMessage
	if err := json.Unmarshal(raw, &spec); err != nil {
		return err
	}
	var paths map[string]map[string]json.RawMessage
	if err := json.Unmarshal(spec["paths"], &paths); err != nil {
		return err
	}
	// Keep the account-level paths of the chosen products ("kv" lives under /storage/kv, "r2" under /r2/buckets).
	var prefixes []string
	for _, product := range strings.Split(products, ",") {
		if under, ok := map[string]string{"kv": "storage/kv", "r2": "r2/buckets"}[product]; ok {
			product = under
		}
		prefixes = append(prefixes, "/accounts/{account_id}/"+product)
	}
	operations := 0
	for path, item := range paths {
		keep := false
		for _, prefix := range prefixes {
			keep = keep || strings.HasPrefix(path, prefix)
		}
		if !keep {
			delete(paths, path)
			continue
		}
		for _, method := range []string{"get", "put", "post", "delete", "patch"} {
			if _, ok := item[method]; ok {
				operations++
			}
		}
	}
	if spec["paths"], err = json.Marshal(paths); err != nil {
		return err
	}
	sliced, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(api, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(api, "openapi.json"), append(sliced, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s/openapi.json: %d operations (%s)\n", api, operations, products)
	if generators := filepath.Join(api, "generators.yml"); !exists(generators) {
		petstore, err := os.ReadFile("sdk/fern/apis/petstore/generators.yml")
		if err != nil {
			return err
		}
		renamed := strings.NewReplacer("/petstore/", "/cloudflare/", "example.com/petstore", "example.com/cloudflare",
			"packageName: petstore", "packageName: cloudflare", "namespaceExport: Petstore", "namespaceExport: Cloudflare").Replace(string(petstore))
		if err := os.WriteFile(generators, []byte(renamed), 0o644); err != nil {
			return err
		}
	}
	fmt.Println("Now: mise run sdk:gen cloudflare go (or typescript, python)")
	return nil
}
