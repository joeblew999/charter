// Command workerbench is go tool charter bench by its old name, kept while repos move to go tool charter (#100).
package main

import (
	"os"

	"github.com/joeblew999/charter/go/lib/internal/cli/bench"
)

func main() { bench.Main(os.Args[1:]) }
