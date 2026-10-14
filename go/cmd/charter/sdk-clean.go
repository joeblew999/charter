package main

import (
	"fmt"
	"os"
	"strings"
)

// charter sdk:clean: remove generated SDKs (sdk/out) and stop leftover WireMock containers
// Run by: mise run sdk:clean (tasks/repo/tasks.toml)
func init() {
	commands["sdk:clean"] = command{"", "remove generated SDKs (sdk/out) and stop leftover WireMock containers", sdkClean}
}

func sdkClean([]string) error {
	for _, dir := range []string{"sdk/out", "sdk/.work"} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if ids, _ := output(".", "docker", "ps", "-q", "--filter", wiremock); ids != "" {
		if err := quiet(".", nil, "docker", append([]string{"rm", "-f"}, strings.Fields(ids)...)...); err != nil {
			return err
		}
		fmt.Println("stopped WireMock containers")
	}
	fmt.Println("clean")
	return nil
}
