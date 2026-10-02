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
	commands["wasm-build"] = command{"[-dir api/go] [-heap 8] [-opt z] [-stack 128kb] [-max 3000000] [-plain]",
		"build a Go Worker's Wasm with TinyGo, tuned for Workers (see docs: Go on Cloudflare Workers), and write the Go library's Worker glue beside it; -plain: TinyGo as it is", wasmBuild}
}

// The changes made to TinyGo, in its runtime source (read at build time: the compiler is not
// rebuilt). Each is a piece of text replaced in a copy of TinyGo's src/.
//
// 1. No collection at every pause. TinyGo runs a full garbage collection whenever the scheduler
// goes idle once 32 finalizers have been registered since the last one. Every JavaScript reference
// registers one, and a Worker goes idle at every await, so one request ran many full collections:
// measured on Cloudflare, 38 to 71 ms of CPU per request against 9 to 13 ms without it
// (docs/benchmarks.md). Zero switches that trigger off; the collector still runs when the heap is
// full, so long-lived streams are still collected.
//
// 2. Goroutine stacks are reused. TinyGo allocates a stack (-stack-size) for every goroutine and
// for every call from JavaScript into Go, and leaves the finished ones to the collector, which
// rarely frees them: a hello on workers-go allocated 1.7 MB of stacks. A finished goroutine's
// stack, which TinyGo has just cleared, now goes on a list, and the next goroutine takes it.
//
// Upstream: tinygo-org/tinygo#5800 (when fixed: drop the first patch)
// Upstream: tinygo-org/tinygo#5801 (closed: fixed on TinyGo's dev branch. The stack patches leave
// themselves out with a TinyGo that has the fix; delete them when this tool pins one)
// The TinyGo this tool builds with, and the wasm-opt TinyGo runs: the compiler and the patches for
// it are one thing, so they are pinned here together, not in a project's mise.toml. wasm-build asks
// mise for exactly these (it installs them the first time). Moving to another TinyGo is a change
// to this tool, tested with its patches, and projects get it by updating the tool.
const (
	tinygoVersion   = "0.42.0"
	binaryenVersion = "133"
)

// tinygoCommand is "tinygo <args>" as mise runs it at the pinned versions; with -tinygo system,
// the tinygo on the path.
func tinygoCommand(system bool, args ...string) (string, []string) {
	if system {
		return "tinygo", args
	}
	return "mise", append([]string{"exec", "tinygo@" + tinygoVersion, "aqua:WebAssembly/binaryen@" + binaryenVersion, "--", "tinygo"}, args...)
}

// A patch replaces old with new in a file of TinyGo's runtime. It is left out when the file
// contains unless: the text of TinyGo's own fix, in a version that has one.
var tinygoPatches = []struct{ file, old, new, unless string }{
	{"src/runtime/gc_finalizer.go", "const finalizerGCThreshold = 32", "const finalizerGCThreshold = 0", ""},
	{"src/internal/task/task_asyncify.go", `	// Create a stack.
	stack := runtime_alloc(stackSize, nil)
`, `	// Take the stack of a finished goroutine (Resume keeps them), or create one.
	stack := freeStack
	if stack != nil && freeStackSize == stackSize {
		freeStack = *(*unsafe.Pointer)(stack)
		*(*unsafe.Pointer)(stack) = nil
	} else {
		stack = runtime_alloc(stackSize, nil)
	}
`, tinygoFreesStacks},
	{"src/internal/task/task_asyncify.go", `		t.clearStack()
		t.state.args = nil
`, `		t.clearStack()
		t.state.args = nil
		// Keep the cleared stack for the next goroutine: a list through the first word of each.
		base := unsafe.Pointer(t.state.canaryPtr)
		if size := uintptr(t.state.top) - uintptr(base); freeStack == nil || size == freeStackSize {
			*(*unsafe.Pointer)(base) = freeStack
			freeStack, freeStackSize = base, size
			t.state.stackState = stackState{}
		}
`, tinygoFreesStacks},
	{"src/internal/task/task_asyncify.go", `// currentTask is the current running task, or nil if currently in the scheduler.
`, `// freeStack is a list of the stacks of finished goroutines, all of freeStackSize bytes.
var (
	freeStack     unsafe.Pointer
	freeStackSize uintptr
)

// currentTask is the current running task, or nil if currently in the scheduler.
`, tinygoFreesStacks},
}

// tinygoFreesStacks is in task_asyncify.go from the TinyGo that frees a finished goroutine's stack
// itself (tinygo-org/tinygo#5801, fixed on its dev branch after 0.42.0).
const tinygoFreesStacks = "runtime_freeTaskStack"

// wasmBuild builds dir's Go program into dir/build/app.wasm for workers-go, and fills dir/build
// with the JavaScript that runs it: workers-go's (wasm_exec.js, runtime.mjs) and the Go library's
// (glue). The Wasm is built:
//   - with the runtime patch above;
//   - with a starting heap of -heap MB, so the collector does not run at all in an ordinary request
//     (TinyGo starts with a heap of a few pages and collects every time it has to grow);
//   - with -stack per goroutine (Huma overflows TinyGo's default 64 KB);
//   - failing over -max bytes gzipped (Workers Free allows 3 MB).
func wasmBuild(args []string) error {
	dir, stack, opt, heap, max, plain, compiler := "api/go", "128kb", "z", 8, 3000000, false, "pinned"
	flags("wasm-build", args, func(f *flag.FlagSet) {
		f.StringVar(&dir, "dir", dir, "the folder of the Go program (its build/ gets the Wasm, workers-go's glue and the library's)")
		f.IntVar(&heap, "heap", heap, "starting heap in MB (0: TinyGo's own, a few pages)")
		f.StringVar(&stack, "stack", stack, "stack per goroutine")
		f.StringVar(&opt, "opt", opt, "TinyGo's optimisation level: z and s for size, 1 and 2 for speed")
		f.IntVar(&max, "max", max, "fail if the Wasm, gzipped, is larger than this many bytes")
		f.StringVar(&compiler, "tinygo", compiler, "pinned: the TinyGo this tool was tested with ("+tinygoVersion+", installed by mise); system: the tinygo on the path, untested")
		f.BoolVar(&plain, "plain", false, "build with TinyGo as it is: no runtime patch, no starting heap (to compare)")
	})
	if err := sh(dir, "go", "run", "github.com/syumai/workers-go/cmd/workers-assets-gen", "-mode=tinygo"); err != nil {
		return err
	}
	if err := glue(dir); err != nil {
		return err
	}
	target, env := "wasm", os.Environ()
	if !plain {
		root, err := tinygoRoot(compiler == "system")
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
	if compiler == "system" {
		fmt.Println("building with the tinygo on the path: this tool is tested with TinyGo " + tinygoVersion)
	}
	name, build := tinygoCommand(compiler == "system", "build", "-o", "build/app.wasm", "-target", target, "-no-debug", "-opt="+opt, "-stack-size="+stack, ".")
	if err := quiet(dir, env[len(os.Environ()):], name, build...); err != nil {
		return err
	}
	return size([]string{"-max", fmt.Sprint(max), filepath.Join(dir, "build", "app.wasm")})
}

// libraryModule is the Go library a Go Worker is built on (go/ in this repo): its packages, and in
// worker/ the JavaScript that the packages transport and hub talk to.
const libraryModule = repoModule + "/go"

// glue writes the library's Worker glue (worker/*.mjs: go.mjs, hub.mjs, websocket.mjs,
// tinygo-clock.mjs) into dir/build, where the project's entry imports it. It is taken from the
// library module as the project in dir resolves it: this repo's go/ through go.work, the module
// cache at the version go.mod pins anywhere else. So the JavaScript is always the one written for
// the Go the project builds against, and no project keeps a copy that can fall behind.
func glue(dir string) error {
	library, err := output(dir, "go", "list", "-m", "-f", "{{.Dir}}", libraryModule)
	if err != nil || library == "" {
		return fmt.Errorf("wasm-build: the Go module in %s does not require %s, which has the Worker glue (go get %s)", dir, libraryModule, libraryModule)
	}
	files, err := filepath.Glob(filepath.Join(library, "worker", "*.mjs"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("wasm-build: no Worker glue (worker/*.mjs) in %s", library)
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if err := write(filepath.Join(dir, "build", filepath.Base(file)), string(content)); err != nil {
			return err
		}
	}
	return nil
}

// tinygoRoot is a TinyGo root whose runtime has the patch: the installed one, with its src/ copied
// and the one line changed. It is made once per TinyGo version and kept in the user's cache folder.
func tinygoRoot(system bool) (string, error) {
	name, args := tinygoCommand(system, "env", "TINYGOROOT")
	installed, err := output(".", name, args...)
	if err != nil {
		return "", errors.New("wasm-build needs mise, to run TinyGo " + tinygoVersion + " (or -tinygo system and a tinygo on the path)")
	}
	installed = strings.TrimSpace(installed[strings.LastIndex(strings.TrimSpace(installed), "\n")+1:])
	name, args = tinygoCommand(system, "version")
	version, _ := output(".", name, args...)
	text := version
	for _, patch := range tinygoPatches {
		text += patch.file + patch.old + patch.new
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))[:12]
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
	for _, patch := range tinygoPatches {
		file := filepath.Join(root, patch.file)
		source, err := os.ReadFile(file)
		if err == nil && patch.unless != "" && strings.Contains(string(source), patch.unless) {
			continue // this TinyGo has the fix itself
		}
		if err != nil || strings.Count(string(source), patch.old) != 1 {
			os.RemoveAll(root)
			return "", fmt.Errorf("this TinyGo (%s) does not have the text a patch changes (%q in %s): see docs/upstream.md, or build with -plain", version, patch.old, patch.file)
		}
		if err := os.WriteFile(file, []byte(strings.Replace(string(source), patch.old, patch.new, 1)), 0o644); err != nil {
			return "", err
		}
	}
	fmt.Printf("TinyGo runtime patched once into %s\n", root)
	return root, os.WriteFile(filepath.Join(root, ".patched"), []byte(version+"\n"), 0o644)
}
