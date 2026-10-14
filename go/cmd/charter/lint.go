package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// charter lint: fail if gofmt would change one of the files (a * pattern is matched here, not by the shell), then go vet the packages (default ./...), and those of -wasm for Wasm (GOOS=js GOARCH=wasm)
// Run by: mise run charter:check (mise.toml); mise run go:check (mise.toml); mise run lint (the examples' mise.toml)
func init() {
	commands["lint"] = command{"[-vet <packages>] [-wasm <packages>] <file or folder>...",
		"fail if gofmt would change one of the files (a * pattern is matched here, not by the shell), then go vet the packages (default ./...), and those of -wasm for Wasm (GOOS=js GOARCH=wasm)", lint}
	anywhere["lint"] = true
}

// lint is the Go lint of a project, or of any folder of Go code: formatted, and vetted for the
// host and, where the code also builds for Workers, for Wasm.
func lint(args []string) error {
	vet, wasm := "./...", ""
	rest := flags("lint", args, func(f *flag.FlagSet) {
		f.StringVar(&vet, "vet", vet, "the packages to go vet, comma-separated")
		f.StringVar(&wasm, "wasm", wasm, "the packages to also go vet for Wasm (GOOS=js GOARCH=wasm), comma-separated")
	})
	if len(rest) == 0 {
		return errors.New("lint needs the files and folders gofmt checks, e.g. lint *.go api cmd")
	}
	var paths []string
	for _, arg := range rest {
		if !strings.ContainsAny(arg, "*?[") {
			paths = append(paths, arg)
			continue
		}
		found, err := filepath.Glob(arg)
		if err != nil || len(found) == 0 {
			return fmt.Errorf("lint: no file matches %s", arg)
		}
		paths = append(paths, found...)
	}
	unformatted, err := output(".", "gofmt", append([]string{"-l"}, paths...)...)
	if err != nil {
		return err
	}
	if unformatted != "" {
		return fmt.Errorf("not formatted (gofmt -w writes them as they should be):\n%s", unformatted)
	}
	if err := sh(".", "go", append([]string{"vet"}, strings.Split(vet, ",")...)...); err != nil {
		return err
	}
	if wasm == "" {
		return nil
	}
	cmd := exec.Command("go", append([]string{"vet"}, strings.Split(wasm, ",")...)...)
	cmd.Env, cmd.Stdout, cmd.Stderr = append(os.Environ(), "GOOS=js", "GOARCH=wasm"), os.Stdout, os.Stderr
	return cmd.Run()
}
