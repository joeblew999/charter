// Package bundle builds a TypeScript (or JavaScript) Worker into build/worker.mjs with esbuild, in this process: one ES
// module, as Cloudflare runs it, from its entry and all it imports (its packages, its JSON), with Cloudflare's own
// modules (cloudflare:*) and Node's (node:fs, or fs) left to the runtime. charter build runs it for a project whose
// charter.toml names an entry ([build] entry), where cf and Vite bundled before (#121).
package bundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// Out is what Build writes, under the project's root: the Worker, its entry module.
const Out = "build/worker.mjs"

// nodeModules are Node's built-in modules (module.builtinModules), which a package may import by their bare name
// (a generated SDK does, behind a check of the runtime it is in).
var nodeModules = strings.Fields(`assert async_hooks buffer child_process cluster console constants crypto dgram
	diagnostics_channel dns domain events fs http http2 https inspector module net os path perf_hooks process punycode
	querystring readline repl stream string_decoder sys timers tls trace_events tty url util v8 vm wasi worker_threads
	zlib`)

// external are the modules left to the runtime: Cloudflare's, and Node's by either name (node:fs, fs, fs/promises),
// which workerd gives a Worker with nodejs_compat.
func external() []string {
	out := []string{"cloudflare:*", "node:*"}
	for _, m := range nodeModules {
		out = append(out, m, m+"/*")
	}
	return out
}

// Build bundles entry (a path under root) into root's build/worker.mjs, and returns its size.
func Build(root, entry string) (int, error) {
	if _, err := os.Stat(filepath.Join(root, entry)); err != nil {
		return 0, fmt.Errorf("the Worker's entry: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
		return 0, err
	}
	result := api.Build(api.BuildOptions{
		EntryPoints:   []string{entry},
		AbsWorkingDir: root,
		Outfile:       Out,
		Write:         true,
		Bundle:        true,
		Format:        api.FormatESModule,
		Platform:      api.PlatformNeutral,
		Target:        api.ES2024,
		// A package's Worker build first, as Cloudflare's tools resolve one (workerd, worker), then the browser's.
		Conditions: []string{"workerd", "worker", "browser"},
		MainFields: []string{"module", "main"},
		External:   external(),
		// As small as a deploy uploads it, names kept so a stack trace reads.
		MinifySyntax:     true,
		MinifyWhitespace: true,
		KeepNames:        true,
		LogLevel:         api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		var msgs []string
		for _, m := range result.Errors {
			where := ""
			if m.Location != nil {
				where = fmt.Sprintf("%s:%d:%d: ", m.Location.File, m.Location.Line, m.Location.Column)
			}
			msgs = append(msgs, where+m.Text)
		}
		return 0, errors.New("bundling the Worker: " + strings.Join(msgs, "\n  "))
	}
	info, err := os.Stat(filepath.Join(root, Out))
	if err != nil {
		return 0, err
	}
	return int(info.Size()), nil
}
