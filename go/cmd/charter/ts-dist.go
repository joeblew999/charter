package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// charter ts:dist: build the TypeScript library (ts/) and pack it, versioned as the tag, into dist/charter-ts-X.Y.Z.tgz: the package a project installs from the GitHub Release's URL (no tag: version 0.0.0-dev)
// Run by: mise run release:publish (mise.toml); mise run ts:dist (mise.toml)
func init() {
	commands["ts:dist"] = command{"[-tag vX.Y.Z]", "build the TypeScript library (ts/) and pack it, versioned as the tag, into dist/charter-ts-X.Y.Z.tgz: the package a project installs from the GitHub Release's URL (no tag: version 0.0.0-dev)", distTS}
	anywhere["ts:dist"] = true
}

// The TypeScript library and the version its package.json carries in the repo, which a release's
// replaces: the package is versioned only as it is packed.
const (
	tsLibrary   = "ts"
	tsDevelop   = `"version": "0.0.0",`
	tsDryRunVer = "0.0.0-dev"
)

// distTS packs the TypeScript library as npm would publish it, without a registry: a tarball for
// the GitHub Release, which a project installs by its URL. It is compiled first (a package in
// node_modules must be JavaScript: Node strips types only outside it). The pack runs on a copy, so
// the version is set without touching the repo's package.json.
func distTS(args []string) error {
	tag, rest, err := releaseTag("dist-ts", args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return errors.New("ts:dist takes no arguments: it packs " + tsLibrary + "/")
	}
	ver := strings.TrimPrefix(tag, "v")
	if ver == "" {
		ver = tsDryRunVer
	}
	manifest, err := os.ReadFile(filepath.Join(tsLibrary, "package.json"))
	if err != nil {
		return fmt.Errorf("ts:dist runs at the root of the charter repo: %w", err)
	}
	if !strings.Contains(string(manifest), tsDevelop) {
		return fmt.Errorf("%s/package.json has no %s line to set the version in", tsLibrary, tsDevelop)
	}
	if err := sh(tsLibrary, "npm", "run", "build"); err != nil {
		return err
	}
	stage, err := os.MkdirTemp("", "charter-ts-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.CopyFS(filepath.Join(stage, "dist"), os.DirFS(filepath.Join(tsLibrary, "dist"))); err != nil {
		return err
	}
	versioned := strings.Replace(string(manifest), tsDevelop, `"version": "`+ver+`",`, 1)
	if err := os.WriteFile(filepath.Join(stage, "package.json"), []byte(versioned), 0o644); err != nil {
		return err
	}
	if license, err := os.ReadFile("LICENSE"); err == nil {
		if err := os.WriteFile(filepath.Join(stage, "LICENSE"), license, 0o644); err != nil {
			return err
		}
	}
	if err := distDir(); err != nil {
		return err
	}
	out, err := filepath.Abs(dist)
	if err != nil {
		return err
	}
	if err := quiet(stage, nil, "npm", "pack", "--pack-destination", out); err != nil {
		return err
	}
	fmt.Printf("packed: %s\n", filepath.Join(dist, "charter-ts-"+ver+".tgz"))
	return nil
}
