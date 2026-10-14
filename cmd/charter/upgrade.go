package main

// charter upgrade: move the repo it is run in to a release of this tool, in one step. The tool's pin
// and every include of charter's tasks name a release and move together, as does the TypeScript
// library a project installs from a release; then AGENTS.md and CLAUDE.md, where they carry its mark,
// are written again as that release writes them. Charter's own places (docs/charter/, its forms, its
// workflow) need nothing here: mise run repo writes them as the release has them.

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
	commands["upgrade"] = command{"[vX.Y.Z]",
		"move the repo you are in to a release of charter, by default the one running: in mise.toml the tool's pin and every include of charter's tasks together, @charter/ts in package.json, and AGENTS.md and CLAUDE.md written again; then mise run repo writes charter's own places as it has them. Run it from the release you want: mise x github:joeblew999/charter@X.Y.Z -- charter upgrade", upgradeCommand}
	anywhere["upgrade"] = true
}

var (
	// The tool's pin under [tools]: "github:joeblew999/charter" = "X.Y.Z".
	pinLine = regexp.MustCompile(`("` + regexp.QuoteMeta(toolRelease) + `"\s*=\s*")[^"]*(")`)
	// An include of charter's tasks from GitHub: git::https://github.com/joeblew999/charter.git//tasks/<folder>?ref=vX.Y.Z.
	includeRef = regexp.MustCompile(`(` + regexp.QuoteMeta("git::"+repoURL+".git//tasks/") + `[^"?]+\?ref=)v[^"]+`)
	// The TypeScript library from a release, as tsPackageURL writes it.
	tsPackage = regexp.MustCompile(regexp.QuoteMeta(repoURL) + `/releases/download/v[^/"]+/charter-ts-[^"]+\.tgz`)
)

func upgradeCommand(args []string) error {
	rest := flags("upgrade", args, func(*flag.FlagSet) {})
	if len(rest) > 1 {
		return errors.New("upgrade takes one release: upgrade [vX.Y.Z]")
	}
	running := toolVersion()
	to := running
	if len(rest) == 1 {
		to = rest[0]
	}
	if to == "" {
		return errors.New("this tool is built from a checkout, not a release: name the release, upgrade vX.Y.Z")
	}
	if !version.MatchString(to) {
		return fmt.Errorf("%q is not a release: give vX.Y.Z", to)
	}
	return upgrade(".", to, to == running, os.Stdout)
}

// upgrade moves root to the release to. What only that release's binary knows - AGENTS.md and
// CLAUDE.md as it writes them - is written only when it is the one running (same); otherwise the
// last line says how to finish.
func upgrade(root, to string, same bool, out *os.File) error {
	say := func(what, file, why string) { fmt.Fprintf(out, "%-8s %s%s\n", what, file, why) }

	file := filepath.Join(root, "mise.toml")
	have, err := os.ReadFile(file)
	if err != nil {
		return errors.New("no mise.toml here: charter adopt (any repo) or charter new (a project) starts one")
	}
	text := string(have)
	moved := pinLine.ReplaceAllString(text, "${1}"+strings.TrimPrefix(to, "v")+"${2}")
	moved = includeRef.ReplaceAllString(moved, "${1}"+to)
	if includeRef.MatchString(moved) {
		moved = withGitIncludes(moved)
	}
	if moved == text {
		say("kept", "mise.toml", ": the tool and charter's tasks are at "+to)
	} else {
		if err := write(file, moved); err != nil {
			return err
		}
		say("wrote", "mise.toml", ": the tool and charter's tasks at "+to)
	}

	if pkg, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil && tsPackage.Match(pkg) {
		if next := tsPackage.ReplaceAllString(string(pkg), tsPackageURL(to)); next != string(pkg) {
			if err := write(filepath.Join(root, "package.json"), next); err != nil {
				return err
			}
			say("wrote", "package.json", ": @charter/ts at "+to+" (then: npm install)")
		}
	}

	if !same {
		fmt.Fprintf(out, "\n  mise install && mise x %s@%s -- charter upgrade    # AGENTS.md and CLAUDE.md as %s writes them\n", toolRelease, strings.TrimPrefix(to, "v"), to)
		return nil
	}
	// Charter's own pointer files, in a repo that is not a project: written again when they differ.
	for name, content := range agentFiles() {
		path := filepath.Join(root, name)
		have, err := os.ReadFile(path)
		if err != nil || !strings.Contains(strings.SplitN(string(have), "\n", 2)[0], "Written by `charter repo`") || string(have) == string(content) {
			continue
		}
		if err := write(path, string(content)); err != nil {
			return err
		}
		say("wrote", name, ": as "+to+" writes it")
	}
	if mod, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil && strings.Contains(string(mod), repoModule+"/go ") {
		say("waiting", "go.mod", ": go get "+repoModule+"/go@"+to+" && go mod tidy")
	}
	fmt.Fprintln(out, "\n  mise install && mise run repo   # the tool at "+to+", then charter's files as it writes them (docs/charter/, the forms, the workflow)")
	return nil
}
