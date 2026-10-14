package main

import (
	"errors"
	"fmt"
)

// charter sdk-check: prove a generated SDK works (sdk/out/<group>): Go = build, vet, tests against WireMock; TypeScript = typecheck
// Run by: mise run sdk:check (tasks/repo/tasks.toml)
func init() {
	commands["sdk-check"] = command{"<group>", "prove a generated SDK works (sdk/out/<group>): Go = build, vet, tests against WireMock; TypeScript = typecheck", sdkCheck}
}

func sdkCheck(args []string) error {
	if len(args) != 1 {
		return errors.New("sdk-check needs <group>, as sdk-gen does: it checks sdk/out/<group>")
	}
	dir := sdkOut(args[0])
	if !exists(dir) {
		return fmt.Errorf("no %s: mise run sdk:gen %s", dir, args[0])
	}
	return sdkCheckDir(dir)
}
