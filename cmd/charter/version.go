package main

import (
	"fmt"
)

// charter version: the release this tool is, which is what new pins a project to
// Run by: people, as `charter version`
func init() {
	commands["version"] = command{"", "the release this tool is, which is what new pins a project to", func([]string) error {
		fmt.Println(orCheckout(toolVersion()))
		return nil
	}}
	anywhere["version"] = true
}
