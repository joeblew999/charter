package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// charter dist-sdk: generate, check and archive the project's SDKs and its specs into dist/ (Docker)
// Run by: mise run sdk:dist (tasks/repo/tasks.toml)
func init() {
	commands["dist-sdk"] = command{"", "generate, check and archive the project's SDKs and its specs into dist/ (Docker)", distSDK}
}

func distSDK(args []string) error {
	if len(args) != 0 {
		return errors.New("dist-sdk takes no arguments: it ships the project's SDKs")
	}
	name, err := distName()
	if err != nil {
		return err
	}
	// Every SDK the project defines ships: a language added to generators.yml is released with no
	// other change. Not the CLI (dist-cli builds it) and not a build made only for the tests.
	groups, err := sdkGroups(generators)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group == "cli" || strings.HasSuffix(group, "-dist") {
			continue
		}
		dir := sdkOut(group)
		// Fresh, so that what ships is what the committed specs give, and proven before it ships.
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		if err := sdkGen([]string{group}); err != nil {
			return err
		}
		if err := sdkCheckDir(dir); err != nil {
			return err
		}
		if err := archive(name+"-sdk-"+group, dir, nil); err != nil {
			return err
		}
	}
	return archive(name+"-specs", fernDir, []string{"openapi.json", "asyncapi.json"})
}

// archive writes dist/<name>.tar.gz with dir's files under <name>/: the files named, or all of
// them except what a build or a check left there.
func archive(name, dir string, files []string) error {
	if len(files) == 0 {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			switch base := entry.Name(); {
			case entry.IsDir() && (base == "node_modules" || base == "target" || base == "target-linux" || base == ".git"):
				return filepath.SkipDir
			case entry.Type().IsRegular() && base != "tsconfig.check.json": // sdk-check writes that one
				rel, err := filepath.Rel(dir, path)
				files = append(files, rel)
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if err := distDir(); err != nil {
		return err
	}
	out, err := os.Create(filepath.Join(dist, name+".tar.gz"))
	if err != nil {
		return err
	}
	defer out.Close()
	zip := gzip.NewWriter(out)
	tape := tar.NewWriter(zip)
	for _, file := range files {
		in, err := os.Open(filepath.Join(dir, file))
		if err != nil {
			return err
		}
		info, err := in.Stat()
		if err != nil {
			return err
		}
		if err := tape.WriteHeader(&tar.Header{Name: name + "/" + filepath.ToSlash(file), Mode: int64(info.Mode().Perm()), Size: info.Size(), ModTime: info.ModTime()}); err != nil {
			return err
		}
		_, err = io.Copy(tape, in)
		in.Close()
		if err != nil {
			return err
		}
	}
	if err := tape.Close(); err != nil {
		return err
	}
	if err := zip.Close(); err != nil {
		return err
	}
	fmt.Printf("archived: %s (%d files)\n", out.Name(), len(files))
	return out.Close()
}
