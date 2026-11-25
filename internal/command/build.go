package command

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/project"
)

const library = "github.com/joeblew999/charter"

// Build runs charter build: everything wrangler needs to run or deploy the Worker. wrangler.jsonc names it as its
// build command, so wrangler dev and wrangler deploy run it themselves.
//
//	generate   the messages (an intl.toml) and the components' Go (a gsx.toml)
//	vite       the browser's files into dist/ (a vite.config.ts)
//	go         the Wasm into build/, with workers-go's JavaScript and charter's glue, which keeps Go runtimes
//	           between requests
//
// -only generate stops after the first: what go test and go vet need.
func Build(args []string) {
	fs := flag.NewFlagSet("charter build", flag.ExitOnError)
	only := fs.String("only", "", `"generate": stop once the Go that go test and go vet need is written`)
	limit := fs.Int("limit", 0, "fail when build/app.wasm is more than this many bytes gzipped (default: charter.toml's [build] limit)")
	fs.Parse(args)
	p := load()
	if *limit == 0 {
		*limit = p.Build.Limit
	}
	check(generate(p))
	if *only == "generate" {
		return
	}
	if hasVite(p) {
		vite, err := npm(p, "vite")
		check(err)
		check(run.Run(p.Root, nil, vite, "build"))
	}
	check(wasm(p, *limit))
}

// generate writes the Go the app's own Go needs before it compiles.
func generate(p *project.Project) error {
	if hasIntl(p) {
		intl.Main([]string{"gen"})
		if exists(filepath.Join(p.Root, "web")) {
			intl.Main([]string{"assets", "-browser", filepath.Join("web", "intl")})
		}
	}
	if hasGSX(p) {
		return run.Run(p.Root, nil, "go", "tool", "gsx", "generate")
	}
	return nil
}

// wasm builds the Worker into build/: workers-go's JavaScript, the Wasm, charter's glue (worker/*.mjs of the charter
// go.mod requires, so the JavaScript is the one written for the Go it runs), and intl's Worker modules.
func wasm(p *project.Project, limit int) error {
	if err := run.Run(p.Root, nil, "go", "run", "github.com/syumai/workers-go/cmd/workers-assets-gen", "-mode=go"); err != nil {
		return err
	}
	out := filepath.Join("build", "app.wasm")
	if err := run.Run(p.Root, map[string]string{"GOOS": "js", "GOARCH": "wasm"}, "go", "build", "-trimpath", "-ldflags=-s", "-o", out, "."); err != nil {
		return err
	}
	dir, err := run.Output(p.Root, nil, "go", "list", "-m", "-f", "{{.Dir}}", library)
	if err != nil {
		return fmt.Errorf("go.mod must require %s: %w", library, err)
	}
	glue, _ := filepath.Glob(filepath.Join(strings.TrimSpace(dir), "worker", "*.mjs"))
	if len(glue) == 0 {
		return fmt.Errorf("no Worker glue (worker/*.mjs) in %s", dir)
	}
	for _, f := range glue {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(p.Root, "build", filepath.Base(f)), b, 0o644); err != nil {
			return err
		}
	}
	// The Worker's entry, in place of workers-go's (a Go runtime per request): charter's go.mjs keeps runtimes between
	// requests, and starts one while the module loads when charter.toml names a health path, which must touch no
	// binding. An app with more to export (a Durable Object) has an entry of its own, importing ./build/go.mjs.
	warm := ""
	if p.Health != "" {
		warm = fmt.Sprintf("await go.warm({ paths: [%q] });\n", p.Health)
	}
	entry := "// Written by charter build: the Worker's entry.\nimport { goWorker } from \"./go.mjs\";\n\nconst go = goWorker();\n" + warm + "\nexport default { fetch: go.fetch };\n"
	if err := os.WriteFile(filepath.Join(p.Root, "build", "worker.mjs"), []byte(entry), 0o644); err != nil {
		return err
	}
	if hasIntl(p) {
		intl.Main([]string{"assets", "-worker", "build"})
	}
	b, err := os.ReadFile(filepath.Join(p.Root, out))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	z, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	z.Write(b)
	z.Close()
	say("✓ %s: %.2f MB, %.2f MB gzipped (Workers Free takes 3, Paid 10)", out, float64(len(b))/1e6, float64(buf.Len())/1e6)
	if limit > 0 && buf.Len() > limit {
		return fmt.Errorf("%s is %d bytes gzipped, over the limit of %d", out, buf.Len(), limit)
	}
	return nil
}
