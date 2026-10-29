// Package projectsteps is a charter project's own steps, beside the Worker's (build, serve, deploy): what a project
// with a contract, specs and SDKs runs, in Go or in TypeScript.
//
//	charter spec [-check]                          both specs from the contract (fern/), Fern's settings, the database's code
//	charter sdk [group] [-prove [group…]] [-publish] an SDK with Fern (Docker) into sdk/out; every client proven; the Go SDK
//	charter dev [-native]                          the Worker on local workerd on charter.toml's port, until Ctrl-C
//	charter live-test                              the standard test and [test] live against the deployed Worker (url)
//	charter check                                  also, for a project: lint, go test, the specs, Fern's check, the
//	                                               Worker tried natively and under workerd ([test]), its size
//
// What a step runs it prints first, and a step of a second or more says how long it took.
package projectsteps

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/joeblew999/charter/project"
)

// load is the project of the working directory, which the step moves to the root of.
func load() *project.Project {
	log.SetFlags(0)
	p, err := project.Load(".")
	check(err)
	check(os.Chdir(p.Root))
	return p
}

func check(err error) {
	if err != nil {
		log.Fatal("✗ ", err)
	}
}

// self is this charter, which a test command named charter runs as.
func self() string {
	bin, err := os.Executable()
	if err != nil {
		return "charter"
	}
	return bin
}

// places are what a test command and [test] env are written with: {base}, the Worker's URL, and {port1}…{port3}.
type places struct {
	base  string
	ports []int
}

var placeholder = regexp.MustCompile(`\{(base|port[1-3])\}`)

func (pl places) fill(s string) string {
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		name := m[1 : len(m)-1]
		if name == "base" {
			return pl.base
		}
		i, _ := strconv.Atoi(strings.TrimPrefix(name, "port"))
		if i-1 < len(pl.ports) {
			return strconv.Itoa(pl.ports[i-1])
		}
		return m
	})
}

// command is a test command's words, filled in: charter is this charter.
func (pl places) command(line string) ([]string, error) {
	words := strings.Fields(pl.fill(line))
	if len(words) == 0 {
		return nil, errors.New("an empty command in [test]")
	}
	if words[0] == "charter" {
		words[0] = self()
	}
	return words, nil
}

// env is [test] env, filled in.
func (pl places) env(p *project.Project) map[string]string {
	out := map[string]string{}
	for k, v := range p.Test.Env {
		out[k] = pl.fill(v)
	}
	return out
}

// localEnv is what the app runs with on this machine: [local] vars, then [test] env over them.
func localEnv(p *project.Project, pl places) map[string]string {
	out := map[string]string{}
	for k, v := range p.Local.Vars {
		out[k] = v
	}
	for k, v := range pl.env(p) {
		out[k] = v
	}
	return out
}

// goModule is the project's Go module (go.mod), "" without one.
func goModule(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	if m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// say prints a line as every step does.
func say(format string, args ...any) { fmt.Printf(format+"\n", args...) }
