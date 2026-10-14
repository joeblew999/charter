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

// charter sdk-publish: generate the Go SDK, check it, and copy its sources into the committed folder another repo can go get (sdk/go); -check: fail if that copy is stale (Docker)
// Run by: mise run sdk:publish (tasks/repo/tasks.toml); mise run sdk:publish:check (tasks/repo/tasks.toml); mise run sdk:publish:fresh (tasks/repo/tasks.toml)
func init() {
	commands["sdk-publish"] = command{"[-check [-quick]] [-into <dir>]", "generate the Go SDK, check it, and copy its sources into the committed folder another repo can go get (sdk/go); -check: fail if that copy is stale (Docker)", sdkPublish}
}

func sdkPublish(args []string) error {
	var check, quick bool
	var into string
	rest := flags("sdk-publish", args, func(f *flag.FlagSet) {
		f.BoolVar(&check, "check", false, "write nothing: fail if the committed copy is not what the specs generate now")
		f.BoolVar(&quick, "quick", false, "with -check: generate nothing (no Docker), only compare the specs with the ones the copy was made from")
		f.StringVar(&into, "into", "sdk/go", "the committed folder: the SDK's module path must end in it")
	})
	if len(rest) != 0 {
		return errors.New("sdk-publish takes no arguments: it publishes the project's go group")
	}
	out := sdkOut("go")
	into = filepath.Clean(into)
	if check && !exists(into) {
		fmt.Printf("%s: not published yet (mise run sdk:publish writes it)\n", into)
		return nil
	}
	source, err := sdkSourceHash()
	if err != nil {
		return err
	}
	if check && quick {
		made, _ := os.ReadFile(filepath.Join(into, sdkMadeFrom))
		if strings.TrimSpace(string(made)) != source {
			return fmt.Errorf("%s was not made from the specs in %s as they are now: mise run sdk:publish writes it again (Docker), then commit it", into, fernDir)
		}
		fmt.Printf("%s was made from the specs in %s as they are now\n", into, fernDir)
		return nil
	}
	// Fresh, so that what is committed is what the committed specs give.
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	if err := sdkGen([]string{"go"}); err != nil {
		return err
	}
	gomod, err := os.ReadFile(filepath.Join(out, "go.mod"))
	if err != nil {
		return err
	}
	module := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(gomod)
	if module == nil || !strings.HasSuffix(string(module[1]), "/"+filepath.ToSlash(into)) {
		return fmt.Errorf("%s/go.mod must declare a module path that ends in /%s, the folder it is committed to: set config.module.path of the go group in %s to <the project's module>/%s",
			out, filepath.ToSlash(into), generators, filepath.ToSlash(into))
	}
	generated, err := sdkSources(out)
	if err != nil {
		return err
	}
	// Said at the top of the copy's README, which is Fern's: nobody wrote this folder by hand.
	generated["README.md"] = append([]byte(fmt.Sprintf("> **Generated code.** Fern generated this Go client from the API's contract (the specs in `%s/`). Don't edit it: change the contract, then `mise run sdk:publish` writes this folder again.\n\n", fernDir)), generated["README.md"]...)
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
			return fmt.Errorf("%s is stale against the specs in %s (mise run sdk:publish writes it again):\n%s", into, fernDir, strings.Join(stale, "\n"))
		}
		fmt.Printf("%s is what the specs in %s generate (%d files)\n", into, fernDir, len(generated))
		return nil
	}
	// Proven before it is committed: build, vet, and Fern's tests against WireMock.
	if err := sdkCheckDir(out); err != nil {
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
	if err := write(filepath.Join(into, sdkMadeFrom), source+"\n"); err != nil {
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
		case entry.Type().IsRegular() && base != "CONTRIBUTING.md" && base != sdkMadeFrom: // how to change generated code: not for this copy
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
