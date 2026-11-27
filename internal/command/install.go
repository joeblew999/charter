package command

import (
	"errors"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/tools"
	"os"
	"path/filepath"
)

// Install runs charter install: everything the app pins, installed: the tools of mise.toml (Go, Node, fnox), npm's
// packages as package-lock.json has them, Go's modules, the tools go.mod names, gsx's language server for the editor,
// the agent skills of everything the app is built with, as Claude Code plugins (skills.go), and a container engine
// (package tools).
func Install(args []string) {
	p := load()
	if tools.Mise() == "" {
		check(errors.New(tools.NoMise))
	}
	check(run.Run(p.Root, nil, tools.Mise(), "install"))
	if exists(filepath.Join(p.Root, "package-lock.json")) {
		check(run.Run(p.Root, nil, "npm", "ci"))
	} else if exists(filepath.Join(p.Root, "package.json")) {
		check(run.Run(p.Root, nil, "npm", "install"))
	}
	check(run.Run(p.Root, nil, "go", "mod", "download"))
	check(agentPlugins(p))
	// gsx's compiler where an editor looks for it (GOBIN, GOPATH/bin), at the version go.mod pins: its language
	// server is gsx lsp, which the editor's gsx extension starts (.vscode/extensions.json recommends it).
	if hasGSX(p) && os.Getenv("CI") == "" {
		check(run.Run(p.Root, nil, "go", "install", "github.com/gsxhq/gsx/cmd/gsx"))
	}
	// A container engine (Kiri, unless one already answers) for the SDK generators: an app of its own, which mise does
	// not install.
	if os.Getenv("CI") == "" {
		_, err := tools.Containers()
		check(err)
	}
}
