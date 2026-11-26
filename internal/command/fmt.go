package command

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Fmt runs charter fmt: Go and gsx files written as gofmt and gsx fmt have them. --check writes nothing, and fails
// where a file would change. gsxui's own vendored files (gsxui.json's ui folder) are gsxui's to format.
func Fmt(args []string) {
	fs := flag.NewFlagSet("charter fmt", flag.ExitOnError)
	checkOnly := fs.Bool("check", false, "write nothing; fail where a file would change")
	fs.Parse(args)
	check(format(load(), !*checkOnly))
}

// format fails on a Go file gofmt would change or a .gsx file gsx fmt would; with write, it writes them instead.
func format(p *project.Project, write bool) error {
	dirs, err := run.Output(p.Root, nil, "go", "list", "-f", "{{.Dir}}", "./...")
	if err != nil {
		return err
	}
	var files []string
	for _, dir := range strings.Fields(dirs) {
		m, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		files = append(files, m...)
	}
	mode := "-l"
	if write {
		mode = "-w"
	}
	out, err := run.Output(p.Root, nil, "gofmt", append([]string{mode}, files...)...)
	if err != nil {
		return err
	}
	if s := strings.TrimSpace(out); s != "" {
		return fmt.Errorf("not formatted (charter fmt writes them as they should be):\n%s", s)
	}
	if !hasGSX(p) {
		return nil
	}
	// The app's own .gsx files: every folder that has some, but the one gsxui vendors its components into.
	var conf struct {
		UI string `json:"ui"`
	}
	if b, err := os.ReadFile(filepath.Join(p.Root, "gsxui.json")); err == nil {
		json.Unmarshal(b, &conf)
	}
	seen := map[string]bool{}
	var own []string
	filepath.WalkDir(p.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(p.Root, path)
		if d.IsDir() {
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "build" || d.Name() == "dist" || (conf.UI != "" && rel == filepath.FromSlash(conf.UI))) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".gsx") && !seen[filepath.Dir(rel)] {
			seen[filepath.Dir(rel)] = true
			own = append(own, filepath.Dir(rel))
		}
		return nil
	})
	if len(own) == 0 {
		return nil
	}
	// gsx fmt -l lists what it would change, and fails when there is any.
	cmd := exec.Command("go", append([]string{"tool", "gsx", "fmt", mode}, own...)...)
	cmd.Dir = p.Root
	if listed, err := cmd.CombinedOutput(); err != nil || (!write && len(bytes.TrimSpace(listed)) > 0) {
		return fmt.Errorf("not formatted (charter fmt writes them as they should be):\n%s", bytes.TrimSpace(listed))
	}
	return nil
}
