package ci

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/joeblew999/charter/go/lib/project"
)

// run is go tool charter ci run. It returns the exit code.
func run(p *project.Project) int {
	var t timings
	defer func() { t.report() }() // the phases as they are when run returns
	onCI, pushed := os.Getenv("CI") != "", os.Getenv("GITHUB_ACTIONS") != "" && os.Getenv("GITHUB_EVENT_NAME") == "push"
	var env []string
	if repo := os.Getenv("GITHUB_REPOSITORY"); os.Getenv("GITHUB_ACTIONS") != "" && repo != "" {
		// go tool charter build shares its builds through the repository's releases (charter-cache): a release, or a
		// change that leaves the Worker's inputs alone, takes the earlier build instead of compiling it again.
		env = append(env, "CHARTER_CACHE_REPO="+repo)
	}
	install := "mise install"
	if onCI && len(p.CI.Tools) > 0 {
		// Only the tools CI runs, and no installing of the rest before each mise task (mise's default).
		install += " " + strings.Join(p.CI.Tools, " ")
		env = append(env, "MISE_TASK_RUN_AUTO_INSTALL=false")
	}
	if !t.phase("tools: "+install, sh(install, env)) {
		return 1
	}
	build := p.CI.Build
	if build == "" {
		build = p.BuildCommand()
	}
	if !t.phase("build: "+build, sh(build, env)) {
		return 1
	}
	// A push publishes the build at once, so Workers Builds deploys its preview while the checks run; a commit that
	// fails them still gets a preview, and this run stays red. A fork's pull request publishes nothing.
	if pushed && !t.phase("publish the build", func() error {
		if err := sh("CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/charter github.com/joeblew999/charter/go/lib/cmd/charter", env)(); err != nil {
			return err
		}
		return sh("bin/charter ci publish", env)()
	}) {
		return 1
	}
	if p.CI.Test != "" && !t.phase("test: "+p.CI.Test, sh(p.CI.Test, env)) {
		return 1
	}
	if version := releaseOf(os.Getenv("GITHUB_REF_TYPE"), os.Getenv("GITHUB_REF_NAME")); pushed && version != "" {
		args := append([]string{"bin/charter", "ci"}, releaseArgs(p.CI.ReleaseHook, version)...)
		if !t.phase("release "+version, command(args, env)) {
			return 1
		}
	}
	return 0
}

// releaseOf is the version a pushed ref releases: a v* tag, or a release/v* branch (which works where only branches
// can be pushed); "" for any other.
func releaseOf(refType, ref string) string {
	switch {
	case refType == "tag" && strings.HasPrefix(ref, "v"):
		return ref
	case refType == "branch" && strings.HasPrefix(ref, "release/v"):
		return strings.TrimPrefix(ref, "release/")
	}
	return ""
}

// sh runs a shell command with the environment and env.
func sh(script string, env []string) func() error {
	return command([]string{"sh", "-c", script}, env)
}

func command(args, env []string) func() error {
	return func() error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout, cmd.Stderr, cmd.Env = os.Stdout, os.Stderr, append(os.Environ(), env...)
		return cmd.Run()
	}
}

// timings records how long each phase took.
type timings []struct {
	name    string
	elapsed time.Duration
	ok      bool
}

// phase runs f as the phase called name and reports whether it worked.
func (t *timings) phase(name string, f func() error) bool {
	fmt.Println("── " + name)
	start := time.Now()
	err := f()
	*t = append(*t, struct {
		name    string
		elapsed time.Duration
		ok      bool
	}{name, time.Since(start), err == nil})
	if err != nil {
		log.Printf("✗ %s: %v", name, err)
	}
	return err == nil
}

// report prints where the time went, and on GitHub writes it to the run's summary.
func (t timings) report() {
	var total time.Duration
	var md strings.Builder
	md.WriteString("| phase | seconds |\n| --- | ---: |\n")
	fmt.Println("\nWhere the time went:")
	for _, p := range t {
		mark := ""
		if !p.ok {
			mark = " ✗"
		}
		s := int(p.elapsed.Round(time.Second).Seconds())
		fmt.Printf("  %-60.60s %5ds%s\n", p.name, s, mark)
		fmt.Fprintf(&md, "| %s%s | %d |\n", p.name, mark, s)
		total += p.elapsed
	}
	fmt.Printf("  %-60s %5ds\n", "total", int(total.Round(time.Second).Seconds()))
	fmt.Fprintf(&md, "| **total** | **%d** |\n", int(total.Round(time.Second).Seconds()))
	if path := os.Getenv("GITHUB_STEP_SUMMARY"); path != "" {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			f.WriteString(md.String())
			f.Close()
		}
	}
}
