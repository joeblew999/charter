package main

import (
	"fmt"
	"os"
)

// charter harness-sync: copy the project's compiled TypeScript SDK (sdk/out/typescript-dist) into its Worker (src/client), generating it first if the specs are newer
// Run by: mise run lint (the examples' mise.toml)
func init() {
	commands["harness-sync"] = command{"", "copy the project's compiled TypeScript SDK (sdk/out/typescript-dist) into its Worker (src/client), generating it first if the specs are newer", harnessSync}
}

func harnessSync([]string) error {
	// Generate when there is no SDK yet, or when a spec or Fern's settings changed after it was made:
	// the specs come from the contract, so a contract change must reach the SDK the test runs.
	const sdk = "sdk/out/typescript-dist/esm/index.mjs"
	stale := !exists(sdk)
	if made, err := os.Stat(sdk); err == nil {
		entries, err := os.ReadDir(fernDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if info, err := entry.Info(); err == nil && info.ModTime().After(made.ModTime()) {
				stale = true
			}
		}
	}
	if stale {
		if err := sdkGen([]string{"typescript-dist"}); err != nil {
			return err
		}
	}
	if err := os.RemoveAll("src/client"); err != nil {
		return err
	}
	if err := os.CopyFS("src/client", os.DirFS("sdk/out/typescript-dist/esm")); err != nil {
		return err
	}
	fmt.Println("SDK copied into src/client")
	return nil
}
