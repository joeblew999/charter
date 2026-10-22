// Package build is go tool charter build (go tool workerbuild is the same command): it builds a Go Worker into build/ with TinyGo, tuned for Cloudflare Workers, and
// fills build/ with what runs it: workers-go's JavaScript and this library's Worker glue (worker/*.mjs,
// go.mjs first). Run it in the Worker's module, with tinygo and wasm-opt on the PATH:
//
// -patches=false, -reuse=false and -heap=0 each turn one of these off, so what each saves can be
// measured (cmd/workerbench, docs/self/benchmarks.md); -compiler=go is a dev build in seconds.
//
//	go tool workerbuild              (go.mod: tool github.com/joeblew999/charter/go/lib/cmd/workerbuild)
//	go run github.com/joeblew999/charter/go/lib/cmd/workerbuild
//
// charter-ts's npm run build runs it too, so a TypeScript-free repo builds its Worker the same way.
//
// The Wasm has TinyGo's runtime patched (patches, below), a starting heap so an ordinary request never
// collects (-heap), a stack per goroutine that Huma fits in (-stack), and the smallest code (-opt z,
// also the quickest build: wasm-opt's asyncify pass takes longer the bigger the Wasm). The same
// sources, pins and options make the same Wasm, so one built before is copied from the cache instead
// (-fresh builds it again). docs/self/benchmarks.md has what each of these saved.
package build

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const library = "github.com/joeblew999/charter/go/lib"

// A change made to TinyGo's runtime source (read at build time: the compiler is not rebuilt).
type patch struct{ file, old, new string }

var patches = []patch{
	// No collection at every pause. TinyGo runs a full collection whenever the scheduler goes idle once
	// 32 finalizers were registered since the last one. Every JavaScript reference registers one, and a
	// Worker goes idle at every await, so it collected on every request: the first step in "What closed
	// the gap" (docs/self/benchmarks.md). The collector still runs when the heap is full.
	// Upstream: tinygo-org/tinygo#5800 (when fixed: drop the patch)
	{"src/runtime/gc_finalizer.go", "const finalizerGCThreshold = 32", "const finalizerGCThreshold = 0"},
}

// Main runs the command with args, the arguments after its name.
func Main(args []string) {
	fs := flag.NewFlagSet("charter build", flag.ExitOnError)
	heapMB := fs.Int("heap", 8, "the Wasm's starting heap, in MB (0: TinyGo's default)")
	opt := fs.String("opt", "z", "TinyGo's -opt")
	stack := fs.String("stack", "128kb", "TinyGo's -stack-size, per goroutine")
	limit := fs.Int("limit", 0, "fail when build/app.wasm is more than this many bytes gzipped (0: no limit)")
	fresh := fs.Bool("fresh", false, "build again even when the cache has this Wasm")
	usePatches := fs.Bool("patches", true, "patch TinyGo's runtime (-patches=false: as TinyGo ships, to measure what the patches save)")
	reuse := fs.Bool("reuse", true, "keep Go runtimes between requests (-reuse=false: one per request, as workers-go's own entry, to measure what reuse saves)")
	compiler := fs.String("compiler", "tinygo", `"tinygo", or "go": the standard compiler, seconds per build, for a dev loop (a bigger Wasm, a runtime per request; never deploy it)`)
	fs.Parse(args)
	if !*usePatches {
		patches = nil
	}
	unlock, err := lock()
	if err != nil {
		fmt.Fprintln(os.Stderr, "workerbuild:", err)
		os.Exit(1)
	}
	defer unlock()
	switch *compiler {
	case "tinygo":
		err = build(*heapMB, *opt, *stack, *limit, *fresh, *reuse)
	case "go":
		err = buildGo()
	default:
		err = fmt.Errorf("-compiler %q: tinygo or go", *compiler)
	}
	if err != nil {
		unlock()
		fmt.Fprintln(os.Stderr, "workerbuild:", err)
		os.Exit(1)
	}
}

func build(heapMB int, opt, stack string, limit int, fresh, reuse bool) error {
	if err := run(nil, "go", "run", "github.com/syumai/workers-go/cmd/workers-assets-gen", "-mode=tinygo"); err != nil {
		return err
	}
	if err := glue(); err != nil {
		return err
	}
	if !reuse {
		if err := noReuse(); err != nil {
			return err
		}
	}
	flags := fmt.Sprintf("heap=%d opt=%s stack=%s", heapMB, opt, stack)
	key, err := buildKey(flags)
	if err != nil {
		return err
	}
	wasm := filepath.Join("build", "app.wasm")
	cached := filepath.Join(cacheDir("wasm"), key+".wasm")
	if _, err := os.Stat(cached); err != nil && !fresh {
		fetchShared(key, cached)
	}
	if _, err := os.Stat(cached); err == nil && !fresh {
		if err := copyFile(cached, wasm); err != nil {
			return err
		}
		fmt.Println("build/app.wasm: built before from the same sources (charter's cache); -fresh builds it again")
	} else {
		root := "" // TinyGo's own, when there is nothing to patch
		if len(patches) > 0 {
			if root, err = tinygoRoot(); err != nil {
				return err
			}
		}
		target := filepath.Join("build", "tinygo-target.json")
		spec := `{ "inherits": ["wasm"] }` + "\n"
		if heapMB > 0 {
			spec = fmt.Sprintf(`{ "inherits": ["wasm"], "ldflags": ["--initial-memory=%d"] }`+"\n", heapMB*1024*1024)
		}
		if err := os.WriteFile(target, []byte(spec), 0o644); err != nil {
			return err
		}
		var env []string
		if root != "" {
			env = []string{"TINYGOROOT=" + root}
		}
		if err := run(env, "tinygo", "build", "-o", wasm, "-target", target, "-no-debug", "-opt="+opt, "-stack-size="+stack, "."); err != nil {
			return err
		}
		if err := copyFile(wasm, cached); err != nil {
			return err
		}
		storeShared(key, wasm)
	}
	raw, gz, err := sizes(wasm)
	if err != nil {
		return err
	}
	fmt.Printf("made %s: %.2f MB, %.2f MB gzipped\n", wasm, float64(raw)/1e6, float64(gz)/1e6)
	if limit > 0 && gz > limit {
		return fmt.Errorf("%s is %d bytes gzipped, over the limit of %d", wasm, gz, limit)
	}
	return nil
}

// noReuse makes build/go.mjs start a Go runtime for every request and keep none, as workers-go's own
// entry does: what reuse saves is then the difference.
func noReuse() error {
	path := filepath.Join("build", "go.mjs")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	const keep = "const MAX_WAITING = 4;"
	if strings.Count(string(b), keep) != 1 {
		return fmt.Errorf("-reuse=false: %s has no %q to change", path, keep)
	}
	return os.WriteFile(path, []byte(strings.Replace(string(b), keep, "const MAX_WAITING = 0; // workerbuild -reuse=false: a runtime per request", 1)), 0o644)
}

// buildGo builds with the standard compiler (GOOS=js GOARCH=wasm), in seconds: for a dev loop, never
// for a deploy. The glue is copied as for TinyGo, but go.mjs is TinyGo's, so build/go.mjs becomes a
// stand-in with its interface (goWorker, warm) over workers-go's own entry, which starts a Go runtime
// per request. A Worker's entry imports build/go.mjs either way.
func buildGo() error {
	if err := run(nil, "go", "run", "github.com/syumai/workers-go/cmd/workers-assets-gen", "-mode=go"); err != nil {
		return err
	}
	wasm := filepath.Join("build", "app.wasm")
	if err := run([]string{"GOOS=js", "GOARCH=wasm"}, "go", "build", "-trimpath", "-ldflags=-s", "-o", wasm, "."); err != nil {
		return err
	}
	if err := glue(); err != nil {
		return err
	}
	standIn := `// workerbuild -compiler=go: a stand-in for go.mjs over workers-go's own entry (a Go runtime per
// request), for a dev build with the standard compiler. Never deploy it.
import worker from "./worker.mjs";
export function goWorker() {
	return { fetch: (request, env, ctx) => worker.fetch(request, env, ctx), warm: async () => {} };
}
`
	if err := os.WriteFile(filepath.Join("build", "go.mjs"), []byte(standIn), 0o644); err != nil {
		return err
	}
	raw, gz, err := sizes(wasm)
	if err != nil {
		return err
	}
	fmt.Printf("made %s (standard Go, for development): %.2f MB, %.2f MB gzipped\n", wasm, float64(raw)/1e6, float64(gz)/1e6)
	return nil
}

// glue writes the library's Worker glue (worker/*.mjs) into build/, from the library as the project's
// go.mod resolves it, so the JavaScript is always the one written for the Go it builds against.
func glue() error {
	dir, err := output(nil, "go", "list", "-m", "-f", "{{.Dir}}", library)
	if err != nil {
		return fmt.Errorf("the project's go.mod must require %s: %w", library, err)
	}
	files, _ := filepath.Glob(filepath.Join(strings.TrimSpace(dir), "worker", "*.mjs"))
	if len(files) == 0 {
		return fmt.Errorf("no Worker glue (worker/*.mjs) in %s", dir)
	}
	for _, f := range files {
		if err := copyFile(f, filepath.Join("build", filepath.Base(f))); err != nil {
			return err
		}
	}
	return nil
}

// buildKey names what goes into the Wasm: Go's and TinyGo's versions, wasm-opt's, the patches and
// flags, go.mod and go.sum, and every package the Worker is built from that is not the standard
// library's, with its Go files (whatever their build tags) and the files it embeds. A module from the
// module cache counts by its path, which has its version.
func buildKey(flags string) (string, error) {
	h := sha256.New()
	for _, cmd := range [][]string{{"go", "version"}, {"tinygo", "version"}, {"wasm-opt", "--version"}} {
		v, err := output(nil, cmd[0], cmd[1:]...)
		if err != nil {
			return "", err
		}
		io.WriteString(h, v)
	}
	fmt.Fprintf(h, "%q\n%s\n", patches, flags)
	for _, name := range []string{"go.mod", "go.sum"} {
		if b, err := os.ReadFile(name); err == nil {
			h.Write(b)
		}
	}
	modcache, err := output(nil, "go", "env", "GOMODCACHE")
	if err != nil {
		return "", err
	}
	modcache = strings.TrimSpace(modcache)
	list, err := output([]string{"GOOS=js", "GOARCH=wasm"}, "go", "list", "-deps", "-tags", "tinygo", "-f",
		`{{if not .Standard}}{{.Dir}}{{"\t"}}{{join .EmbedFiles " "}}{{end}}`, ".")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(list), "\n")
	slices.Sort(lines)
	for _, line := range lines {
		dir, embeds, _ := strings.Cut(line, "\t")
		if rel, err := filepath.Rel(modcache, dir); err == nil && !strings.HasPrefix(rel, "..") {
			fmt.Fprintf(h, "%s\n", filepath.ToSlash(rel))
			continue
		}
		rel, _ := filepath.Rel(".", dir)
		fmt.Fprintf(h, "%s\n", filepath.ToSlash(rel))
		var files []string
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".go") {
				files = append(files, e.Name())
			}
		}
		files = append(files, strings.Fields(embeds)...)
		slices.Sort(files)
		for _, name := range files {
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return "", err
			}
			fmt.Fprintf(h, "%s\n", name)
			h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:24], nil
}

// tinygoRoot is a TinyGo root whose runtime has the patches: the installed one with its src/ copied
// and the patched files changed, made once per TinyGo version and kept in the cache. What is not src/
// is a link to the installed one.
func tinygoRoot() (string, error) {
	env, err := output(nil, "tinygo", "env", "TINYGOROOT")
	if err != nil {
		return "", err
	}
	envLines := strings.Split(strings.TrimSpace(env), "\n")
	installed := strings.TrimSpace(envLines[len(envLines)-1])
	version, err := output(nil, "tinygo", "version")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s%q", version, patches)))
	root := filepath.Join(cacheDir(), "tinygo-"+hex.EncodeToString(sum[:])[:12])
	if _, err := os.Stat(filepath.Join(root, ".patched")); err == nil {
		return root, nil
	}
	os.RemoveAll(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(installed)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		from, to := filepath.Join(installed, e.Name()), filepath.Join(root, e.Name())
		if e.Name() == "src" {
			// TinyGo wants src/ to be real folders, and the patches change files in it. On macOS a clone.
			if runtime.GOOS != "darwin" || exec.Command("cp", "-c", "-R", from, to).Run() != nil {
				if err := os.CopyFS(to, os.DirFS(from)); err != nil {
					return "", err
				}
			}
		} else if os.Symlink(from, to) != nil {
			if err := os.CopyFS(to, os.DirFS(from)); err != nil { // Windows lets only some users make links
				return "", err
			}
		}
	}
	for _, pt := range patches {
		file := filepath.Join(root, pt.file)
		b, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		source := string(b)
		old, replacement := pt.old, pt.new
		if strings.Contains(source, "\r\n") {
			old, replacement = strings.ReplaceAll(old, "\n", "\r\n"), strings.ReplaceAll(replacement, "\n", "\r\n")
		}
		if strings.Count(source, old) != 1 {
			os.RemoveAll(root)
			return "", fmt.Errorf("this TinyGo (%s) does not have the text a patch changes (%q in %s): see https://github.com/joeblew999/charter/blob/main/docs/self/upstream.md", strings.TrimSpace(version), pt.old, pt.file)
		}
		os.Remove(file) // a new file, not new content in the installed one's (a hard link on Windows)
		if err := os.WriteFile(file, []byte(strings.Replace(source, old, replacement, 1)), 0o644); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".patched"), []byte(version), 0o644); err != nil {
		return "", err
	}
	fmt.Println("TinyGo runtime patched once into", root)
	return root, nil
}

// cacheDir is charter's cache folder (the same one charter-ts uses), with parts under it.
func cacheDir(parts ...string) string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(append([]string{base, "charter"}, parts...)...)
	os.MkdirAll(dir, 0o755)
	return dir
}

func run(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func output(env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}

func sizes(path string) (raw, gzipped int, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	w.Write(b)
	w.Close()
	return len(b), buf.Len(), nil
}
