package main

import (
	"errors"
	"fmt"
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
}

// A tag is a comment line such as:  Upstream: owner/repo#123 (when fixed: what to do then)
var upstreamTag = regexp.MustCompile(`^([^:]+:[0-9]+):.*Upstream: ([\w.-]+/[\w.-]+)#([0-9]+) *(.*)$`)

// upstream finds the tags and asks GitHub for each issue's state. CLOSED means that workaround can
// go (the table in docs/upstream.md says what to do).
func upstream([]string) error {
	out, err := output(".", "git", "grep", "-n", "-E", `Upstream: [A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#[0-9]+`, "--", ":!*.md", ":!mise.toml", ":!dev/misc.go", ":!dev/*_test.go")
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
	for _, dir := range []string{"api/ts", "api/go", "sdk", "sdk/harness"} {
		if !exists(dir) {
			continue // a project made by `dev new` has only api/go and sdk
		}
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
		{"go", "version", "api/go, dev, sdk:check on Go SDKs"},
		{"tinygo", "version", "api:go:build"},
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
