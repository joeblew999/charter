package main

import (
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func init() {
	commands["wasm-build"] = command{"[-dir api-go] [-heap 8] [-opt z] [-stack 256kb] [-max 3000000] [-plain]",
		"build a Go Worker's Wasm with TinyGo, tuned for Workers (see docs: Go on Cloudflare Workers); -plain: TinyGo as it is", wasmBuild}
}

// The one change made to TinyGo, in its runtime source (read at build time: the compiler is not rebuilt).
//
// TinyGo runs a full garbage collection whenever the scheduler goes idle once 32 finalizers have
// been registered since the last one. Every JavaScript reference registers one, and a Worker goes
// idle at every await, so one request ran many full collections: measured on Cloudflare, 38 to 71 ms
// of CPU per request against 9 to 13 ms without it (docs/benchmarks.md). Zero switches that trigger
// off; the collector still runs when the heap is full, so long-lived streams are still collected.
//
// Upstream: tinygo-org/tinygo#5800 (when fixed: drop the patch and build with TinyGo as it is)
const (
	tinygoPatchFile = "src/runtime/gc_finalizer.go"
	tinygoPatchOld  = "const finalizerGCThreshold = 32"
	tinygoPatchNew  = "const finalizerGCThreshold = 0"
)

// wasmBuild builds dir's Go program into dir/build/app.wasm for workers-go:
//   - with the runtime patch above;
//   - with a starting heap of -heap MB, so the collector does not run at all in an ordinary request
//     (TinyGo starts with a heap of a few pages and collects every time it has to grow);
//   - with -stack per goroutine (Huma overflows TinyGo's default 64 KB);
//   - failing over -max bytes gzipped (Workers Free allows 3 MB).
func wasmBuild(args []string) error {
	dir, stack, opt, heap, max, plain := "api-go", "256kb", "z", 8, 3000000, false
	flags("wasm-build", args, func(f *flag.FlagSet) {
		f.StringVar(&dir, "dir", dir, "the folder of the Go program (its build/ gets the Wasm and workers-go's glue)")
		f.IntVar(&heap, "heap", heap, "starting heap in MB (0: TinyGo's own, a few pages)")
		f.StringVar(&stack, "stack", stack, "stack per goroutine")
		f.StringVar(&opt, "opt", opt, "TinyGo's optimisation level: z and s for size, 1 and 2 for speed")
		f.IntVar(&max, "max", max, "fail if the Wasm, gzipped, is larger than this many bytes")
		f.BoolVar(&plain, "plain", false, "build with TinyGo as it is: no runtime patch, no starting heap (to compare)")
	})
	if err := sh(dir, "go", "run", "github.com/syumai/workers-go/cmd/workers-assets-gen", "-mode=tinygo"); err != nil {
		return err
	}
	target, env := "wasm", os.Environ()
	if !plain {
		root, err := tinygoRoot()
		if err != nil {
			return err
		}
		env = append(env, "TINYGOROOT="+root)
		if heap > 0 {
			target = filepath.Join("build", "tinygo-target.json")
			spec := fmt.Sprintf("{ \"inherits\": [\"wasm\"], \"ldflags\": [\"--initial-memory=%d\"] }\n", heap*1024*1024)
			if err := os.WriteFile(filepath.Join(dir, target), []byte(spec), 0o644); err != nil {
				return err
			}
		}
	}
	if err := quiet(dir, env[len(os.Environ()):], "tinygo", "build", "-o", "build/app.wasm", "-target", target, "-no-debug", "-opt="+opt, "-stack-size="+stack, "."); err != nil {
		return err
	}
	return size([]string{"-max", fmt.Sprint(max), filepath.Join(dir, "build", "app.wasm")})
}

// tinygoRoot is a TinyGo root whose runtime has the patch: the installed one, with its src/ copied
// and the one line changed. It is made once per TinyGo version and kept in the user's cache folder.
func tinygoRoot() (string, error) {
	installed, err := output(".", "tinygo", "env", "TINYGOROOT")
	if err != nil {
		return "", errors.New("wasm-build needs tinygo (mise install)")
	}
	version, _ := output(".", "tinygo", "version")
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(version+tinygoPatchOld+tinygoPatchNew)))[:12]
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(cache, "orpc-api", "tinygo-"+key)
	if exists(filepath.Join(root, ".patched")) {
		return root, nil
	}
	if err := os.RemoveAll(root); err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(installed)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Name() == "src" {
			continue
		}
		if err := os.Symlink(filepath.Join(installed, entry.Name()), filepath.Join(root, entry.Name())); err != nil {
			return "", err
		}
	}
	// TinyGo wants src/ to be real folders. On macOS the copy is a clone (no extra disk).
	copyArgs := []string{"-R", filepath.Join(installed, "src"), filepath.Join(root, "src")}
	if runtime.GOOS == "darwin" {
		copyArgs = append([]string{"-c"}, copyArgs...)
	}
	if err := quiet(".", nil, "cp", copyArgs...); err != nil {
		return "", err
	}
	file := filepath.Join(root, tinygoPatchFile)
	source, err := os.ReadFile(file)
	if err != nil || strings.Count(string(source), tinygoPatchOld) != 1 {
		os.RemoveAll(root)
		return "", fmt.Errorf("this TinyGo (%s) does not have the line the patch changes (%q in %s): see docs/upstream.md, or build with -plain", version, tinygoPatchOld, tinygoPatchFile)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(source), tinygoPatchOld, tinygoPatchNew, 1)), 0o644); err != nil {
		return "", err
	}
	fmt.Printf("TinyGo runtime patched once into %s\n", root)
	return root, os.WriteFile(filepath.Join(root, ".patched"), []byte(version+"\n"), 0o644)
}
