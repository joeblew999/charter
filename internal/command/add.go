package command

import (
	"errors"
	"fmt"
	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/run"
	"slices"
	"strings"
)

// Add runs charter add <what>: something more for the app.
//
//	charter add locale:de            a language: charter.toml's [intl], and its catalog to translate
//	charter add ui:dialog            a gsxui component (gsxui add)
//	charter add screen:language-list one of intl's screens, as gsx to own (-diff: what changed; -overwrite: again)
//	charter add d1:DB                a D1 database, created on Cloudflare and bound in wrangler.jsonc
//	charter add github.com/x/y       a Go module (go get)
//	charter add leaflet              an npm package (npm install)
func Add(args []string) {
	if len(args) == 0 {
		check(errors.New("usage: charter add locale:<id> | ui:<component> | screen:<name> | d1:<binding> | <Go module> | <npm package>"))
	}
	p := load()
	// Flags are for the screens (charter add screen:preferences -overwrite, or -diff).
	var flags []string
	args = slices.DeleteFunc(args, func(a string) bool {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			return true
		}
		return false
	})
	for _, what := range args {
		kind, name, typed := strings.Cut(what, ":")
		switch {
		case typed && kind == "locale":
			if !hasIntl(p) {
				intl.Main([]string{"init", "-locales", "en," + name})
			} else {
				intl.Main([]string{"add-locale", name})
			}
		case typed && kind == "ui":
			check(run.Run(p.Root, nil, "go", "tool", "gsxui", "add", name))
		case typed && kind == "screen":
			intl.Main(append(append([]string{"add"}, flags...), name))
		case typed && kind == "d1":
			check(cloudflare(p, []string{"d1", "create", p.Worker.Name + "-" + strings.ToLower(name), "--binding", name, "--update-config"}, nil))
		case typed:
			check(fmt.Errorf("charter add %s: locale:, ui:, screen: or d1:", what))
		case modulePath.MatchString(what):
			check(run.Run(p.Root, nil, "go", "get", what))
		default:
			check(run.Run(p.Root, nil, "npm", "install", what))
		}
	}
}
