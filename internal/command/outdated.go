package command

import (
	"fmt"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Outdated runs charter outdated: every pin with a newer version, Go's and npm's.
func Outdated(args []string) {
	p := load()
	out, err := run.Output(p.Root, nil, "go", "list", "-m", "-u", "-f", "{{if and (not .Indirect) .Update}}{{.Path}} {{.Version}} → {{.Update.Version}}{{end}}", "all")
	check(err)
	if s := strings.TrimSpace(out); s != "" {
		fmt.Println("Go modules:\n" + s)
	} else {
		fmt.Println("Go modules: every one is the latest")
	}
	if exists(filepath.Join(p.Root, "package.json")) {
		fmt.Println("\nnpm packages:")
		cmd := exec.Command("npm", "outdated") // exits 1 when something is outdated
		cmd.Dir, cmd.Stdout, cmd.Stderr = p.Root, os.Stdout, os.Stderr
		if cmd.Run() == nil {
			fmt.Println("every one is the latest its range allows")
		}
	}
}
