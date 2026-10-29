// Package local is go tool charter local, serve and with: the Worker on plain workerd from charter.toml, no wrangler
// and no Node (internal/local writes what workerd runs into .charter/).
//
//	go tool charter local                  write .charter/ (after a build)
//	go tool charter serve                  run it on charter.toml's port (PORT overrides), in the developer's time zone
//	go tool charter with [-strict] cmd…    run cmd against it on a free port with its own state, BASE its URL, then
//	                                       stop it; -strict also fails on a panic or uncaught exception in its log
package local

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/joeblew999/charter/internal/local"
	"github.com/joeblew999/charter/project"
)

// Local runs go tool charter local.
func Local(args []string) {
	fs := flag.NewFlagSet("charter local", flag.ExitOnError)
	fs.Parse(args)
	p := load()
	check(local.Write(p))
	log.Printf("✓ %s: workerd serve %s/config.capnp", filepath.Join(p.Root, local.Dir), local.Dir)
}

// Serve runs go tool charter serve: workerd in the foreground, until it is stopped.
func Serve(args []string) {
	fs := flag.NewFlagSet("charter serve", flag.ExitOnError)
	fs.Parse(args)
	p := load()
	check(local.Write(p))
	bin, err := exec.LookPath("workerd")
	check(err)
	check(os.Chdir(p.Root))
	log.Printf("→ http://127.0.0.1:%s (workerd, in %s)", p.LocalPort(), Zone())
	env := append(os.Environ(), "LOCAL_TIME_ZONE="+Zone())
	check(becomeProcess(bin, []string{"workerd", "serve", local.Dir + "/config.capnp", "--socket-addr", "http=127.0.0.1:" + p.LocalPort()}, env))
}

// With runs go tool charter with.
func With(args []string) {
	fs := flag.NewFlagSet("charter with", flag.ExitOnError)
	strict := fs.Bool("strict", false, "also fail when workerd's log has a panic or an uncaught exception")
	logPath := fs.String("log", "", "workerd's log (default .charter/workerd-<port>.log)")
	fs.Parse(args)
	if fs.NArg() == 0 {
		log.Fatal("usage: go tool charter with [-strict] [-log file] <command> [args…]")
	}
	p := load()
	check(local.Write(p))
	os.Exit(with(p, *strict, *logPath, fs.Args()))
}

func with(p *project.Project, strict bool, logPath string, command []string) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	check(err)
	port := ln.Addr().(*net.TCPAddr).Port
	socket, err := ln.(*net.TCPListener).File()
	check(err)
	ln.Close()
	state, err := os.MkdirTemp("", "charter-state-")
	check(err)
	defer os.RemoveAll(state)
	if logPath == "" {
		logPath = filepath.Join(p.Root, local.Dir, fmt.Sprintf("workerd-%d.log", port))
	}
	out, err := os.Create(logPath)
	check(err)
	defer out.Close()

	workerd := exec.Command("workerd", "serve", local.Dir+"/config.capnp", "--socket-fd", "http=3", "--directory-path", "state="+state)
	workerd.Dir, workerd.Stdout, workerd.Stderr = p.Root, out, out
	workerd.ExtraFiles = []*os.File{socket}
	check(workerd.Start())
	socket.Close()
	exited := make(chan error, 1)
	go func() { exited <- workerd.Wait() }()
	defer stop(workerd, exited)

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := healthy(base+p.Health, exited); err != nil {
		b, _ := os.ReadFile(logPath)
		log.Printf("✗ workerd: %v\n%s", err, b)
		return 1
	}
	log.Printf("→ %s (workerd, log %s)", base, logPath)

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "BASE="+base)
	status := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			log.Printf("✗ %s: %v", command[0], err)
			return 1
		}
		status = exit.ExitCode()
	}
	if strict {
		if b, _ := os.ReadFile(logPath); regexp.MustCompile(`(?i)panic|uncaught`).Match(b) {
			log.Printf("✗ errors in %s:\n%s", logPath, b)
			status = 1
		}
	}
	return status
}

// healthy waits for url to answer 2xx, up to 20 s, or for workerd to exit.
func healthy(url string, exited <-chan error) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			return fmt.Errorf("exited: %v", err)
		default:
		}
		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			if resp.StatusCode/100 == 2 {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("%s didn't answer 2xx in 20 s", url)
}

// stop ends workerd: SIGTERM, and SIGKILL after 3 s (it can wait on hibernated WebSockets while it drains).
func stop(workerd *exec.Cmd, exited <-chan error) {
	workerd.Process.Signal(syscall.SIGTERM)
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		workerd.Process.Kill()
		<-exited
	}
}

// Zone is the machine's IANA time zone, which serve gives the app as Cloudflare would (request.cf.timezone): TZ, else
// what /etc/localtime points at (macOS, most Linux), else /etc/timezone, else UTC.
func Zone() string {
	if tz := strings.TrimPrefix(os.Getenv("TZ"), ":"); tz != "" {
		return tz
	}
	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if _, zone, ok := strings.Cut(target, "zoneinfo/"); ok {
			return zone
		}
	}
	if b, err := os.ReadFile("/etc/timezone"); err == nil && strings.TrimSpace(string(b)) != "" {
		return strings.TrimSpace(string(b))
	}
	return "UTC"
}

func load() *project.Project {
	log.SetFlags(0)
	p, err := project.Load(".")
	check(err)
	if p.Health == "" {
		p.Health = "/"
	}
	return p
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
