// Command workerdeploy is go tool charter deploy by its old name, kept while repos move to go tool charter (#100).
package main

import (
	"os"

	"github.com/joeblew999/charter/go/lib/internal/cli/deploy"
)

func main() { deploy.Main(os.Args[1:]) }
