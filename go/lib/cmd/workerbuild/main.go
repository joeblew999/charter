// Command workerbuild is go tool charter build by its old name, kept while repos move to go tool charter (#100).
package main

import (
	"os"

	"github.com/joeblew999/charter/go/lib/internal/cli/build"
)

func main() { build.Main(os.Args[1:]) }
