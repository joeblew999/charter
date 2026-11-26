package command

import (
	"errors"
	"fmt"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"path/filepath"
	"regexp"
)

// Remove runs charter remove <what>: a Go module or an npm package the app no longer uses.
func Remove(args []string) {
	if len(args) == 0 {
		check(errors.New("usage: charter remove <Go module> | <npm package>"))
	}
	p := load()
	for _, what := range args {
		if modulePath.MatchString(what) {
			// The requirement is dropped and go mod tidy decides the rest. (go get <module>@none would also move
			// every module that needs it to a version that did not, years back.)
			check(run.Run(p.Root, nil, "go", "mod", "edit", "-droprequire", what))
			check(run.Run(p.Root, nil, "go", "mod", "tidy"))
			if b, err := os.ReadFile(filepath.Join(p.Root, "go.mod")); err == nil && regexp.MustCompile(`(?m)^\s*(require\s+)?`+regexp.QuoteMeta(what)+` \S+$`).Match(b) {
				check(fmt.Errorf("%s is still imported by the app's code: go mod tidy put it back", what))
			}
		} else {
			check(run.Run(p.Root, nil, "npm", "uninstall", what))
		}
	}
}
