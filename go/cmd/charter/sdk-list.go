package main

import (
	"fmt"
	"strings"
)

// charter sdk:list: the SDK groups the project defines (fern/generators.yml)
// Run by: mise run sdk:list (tasks/repo/tasks.toml)
func init() {
	commands["sdk:list"] = command{"", "the SDK groups the project defines (fern/generators.yml)", sdkList}
}

func sdkList([]string) error {
	groups, err := sdkGroups(generators)
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(groups, " "))
	return nil
}
