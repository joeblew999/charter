package bundle

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joeblew999/charter/internal/fern"
	"github.com/joeblew999/charter/project"
)

// EnvFile is where Prepare writes the env's types, under the project's root: in .charter/ (charter's, ignored by git), which a
// TypeScript project's tsconfig.json includes.
const EnvFile = ".charter/env.d.ts"

// Env is the Worker's env as Cloudflare gives it, typed: each binding of charter.toml, as an interface the runtime
// types (@cloudflare/workers-types) leave to the Worker (Cloudflare.Env), the type of cloudflare:workers' env and of the
// env a handler is given.
func Env(p *project.Project) (string, error) {
	fields := map[string]string{}
	vars := []map[string]string{p.Production.Vars, p.Local.Vars}
	for _, e := range []*project.Environment{p.Staging, p.Preview} {
		if e != nil {
			vars = append(vars, e.Vars)
		}
	}
	for _, v := range vars {
		for k := range v {
			fields[k] = "string"
		}
	}
	for _, k := range p.Secrets.Required {
		fields[k] = "string"
	}
	for _, k := range p.Secrets.Optional {
		fields[k+"?"] = "string"
		delete(fields, k)
	}
	for _, d := range p.D1 {
		fields[d.Binding] = "D1Database"
	}
	for _, d := range p.DurableObjects {
		fields[d.Binding] = "DurableObjectNamespace"
	}
	limits, err := p.Limits()
	if err != nil {
		return "", err
	}
	for _, r := range limits {
		fields[r.Binding] = "RateLimit"
	}
	var b strings.Builder
	b.WriteString("// Written by `charter build` from charter.toml: don't edit it, change charter.toml. The Worker's env as Cloudflare\n" +
		"// gives it, each binding charter.toml declares: cloudflare:workers' env, and the env a handler is given.\n" +
		"declare namespace Cloudflare {\n\tinterface Env {\n")
	for _, k := range slices.Sorted(maps.Keys(fields)) {
		fmt.Fprintf(&b, "\t\t%s: %s;\n", k, fields[k])
	}
	b.WriteString("\t}\n}\ninterface Env extends Cloudflare.Env {}\n")
	return b.String(), nil
}

// Prepare readies a TypeScript Worker's sources for tsc and Build: the SDK it runs copied in ([sdks] in_worker), and
// its env's types written (tsconfig.json includes them).
func Prepare(p *project.Project) error {
	if p.SDKs.InWorker != "" {
		if err := sdkInWorker(p); err != nil {
			return err
		}
	}
	return writeEnv(p)
}

// sdkInWorker copies the compiled TypeScript SDK into the folder [sdks] in_worker names, for a Worker that runs it,
// generating it first when the specs are newer than what made it.
func sdkInWorker(p *project.Project) error {
	if _, err := fern.Generate(p.Root, "typescript-dist", true, false); err != nil {
		return err
	}
	to := filepath.Join(p.Root, p.SDKs.InWorker)
	if err := os.RemoveAll(to); err != nil {
		return err
	}
	if err := os.CopyFS(to, os.DirFS(filepath.Join(p.Root, "sdk", "out", "typescript-dist", "esm"))); err != nil {
		return err
	}
	fmt.Printf("SDK copied into %s\n", p.SDKs.InWorker)
	return nil
}

// writeEnv writes Env into the project's .charter/env.d.ts, unless it holds it already.
func writeEnv(p *project.Project) error {
	text, err := Env(p)
	if err != nil {
		return err
	}
	path := filepath.Join(p.Root, EnvFile)
	if old, err := os.ReadFile(path); err == nil && string(old) == text {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}
