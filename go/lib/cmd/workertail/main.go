// Command workertail is go tool charter tail by its old name, kept while repos move to go tool charter (#100).
package main

import (
	"os"

	"github.com/joeblew999/charter/go/lib/internal/cli/tail"
)

func main() { tail.Main(os.Args[1:]) }
