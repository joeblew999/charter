package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// A generated SDK lives in sdk/out, which git ignores, so no other repo can `go get` it. sdk-publish
// copies the Go SDK's sources into a committed folder whose path is the end of the SDK's module path
// (sdk/go for <repo module>/sdk/go, set in the API's generators.yml). Go then finds it like any module
// in a subdirectory: at a branch or commit straight away, at a version once `release-tags` has tagged it.

func init() {
	commands["sdk-publish"] = command{"[-check] [-into <dir>] <api>", "generate an API's Go SDK, check it, and copy its sources into the committed folder another repo can go get (sdk/go); -check: fail if that copy is stale (Docker)", sdkPublish}
}

func sdkPublish(args []string) error {
	var check bool
	var into string
	rest := flags("sdk-publish", args, func(f *flag.FlagSet) {
		f.BoolVar(&check, "check", false, "write nothing: fail if the committed copy is not what the specs generate now")
		f.StringVar(&into, "into", "sdk/go", "the committed folder: the SDK's module path must end in it")
	})
	if len(rest) != 1 {
		return errors.New("sdk-publish needs <api>: a folder in sdk/fern/apis with a go group")
	}
	api, out := rest[0], filepath.Join("sdk/out", rest[0], "go")
	into = filepath.Clean(into)
	if check && !exists(into) {
		fmt.Printf("%s: not published yet (mise run sdk:publish writes it)\n", into)
		return nil
	}
	// Fresh, so that what is committed is what the committed specs give.
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	if err := sdkGen([]string{api, "go"}); err != nil {
		return err
	}
	gomod, err := os.ReadFile(filepath.Join(out, "go.mod"))
	if err != nil {
		return err
	}
	module := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(gomod)
	if module == nil || !strings.HasSuffix(string(module[1]), "/"+filepath.ToSlash(into)) {
		return fmt.Errorf("%s/go.mod must declare a module path that ends in /%s, the folder it is committed to: set config.module.path of the go group in sdk/fern/apis/%s/generators.yml to <this repo's module>/%s",
			out, filepath.ToSlash(into), api, filepath.ToSlash(into))
	}
	generated, err := sdkSources(out)
	if err != nil {
		return err
	}
	if check {
		committed, err := sdkSources(into)
		if err != nil {
			return err
		}
		var stale []string
		for file, content := range generated {
			if have, ok := committed[file]; !ok {
				stale = append(stale, "  missing: "+file)
			} else if !bytes.Equal(have, content) {
				stale = append(stale, "  differs: "+file)
			}
		}
		for file := range committed {
			if _, ok := generated[file]; !ok {
				stale = append(stale, "  not generated: "+file)
			}
		}
		if len(stale) > 0 {
			sort.Strings(stale)
			return fmt.Errorf("%s is stale against sdk/fern/apis/%s (mise run sdk:publish writes it again):\n%s", into, api, strings.Join(stale, "\n"))
		}
		fmt.Printf("%s is what sdk/fern/apis/%s generates (%d files)\n", into, api, len(generated))
		return nil
	}
	// Proven before it is committed: build, vet, and Fern's tests against WireMock.
	if err := sdkCheck([]string{out}); err != nil {
		return err
	}
	if err := os.RemoveAll(into); err != nil {
		return err
	}
	size := 0
	for file, content := range generated {
		if err := write(filepath.Join(into, file), string(content)); err != nil {
			return err
		}
		size += len(content)
	}
	// In the workspace, so `go build`, `go vet` and `go test` work inside it like in any module here.
	if err := quiet(".", nil, "go", "work", "use", "./"+filepath.ToSlash(into)); err != nil {
		return err
	}
	fmt.Printf("published: %s (module %s, %d files, %d KB). Commit it; another repo gets it with: go get %s@<branch, commit or version>\n",
		into, module[1], len(generated), (size+1023)/1024, module[1])
	return nil
}

// sdkSources are the files of a Go SDK that get committed, by path: what a program that imports it
// needs, and the tests that run on their own. Left out: Fern's run record (.fern, it names the commit
// it ran at), and WireMock's fixtures with the tests that need its container (wiremock/, */*_test/).
func sdkSources(dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch base := entry.Name(); {
		case entry.IsDir() && (base == ".fern" || base == "wiremock" || strings.HasSuffix(base, "_test")):
			return filepath.SkipDir
		case entry.Type().IsRegular() && base != "CONTRIBUTING.md": // how to change generated code: not for this copy
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)], err = os.ReadFile(path)
			return err
		}
		return nil
	})
	return files, err
}
