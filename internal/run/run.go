// Package run runs what a project's steps run: a program from the folder's packages or the PATH, its output shown
// or kept, a server started, waited for and stopped around what uses it, and how long each took; and charter's
// cache, where a step keeps what it may reuse. The steps print each command before it runs.
package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Bin is a program as the folder's packages install it (node_modules/.bin, in dir or a workspace above it), else
// its name, which the PATH finds.
func Bin(program, dir string) string {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return program
	}
	for d := abs; ; d = filepath.Dir(d) {
		local := filepath.Join(d, "node_modules", ".bin", program)
		if runtime.GOOS == "windows" {
			local += ".cmd"
		}
		if _, err := os.Stat(local); err == nil {
			return local
		}
		if filepath.Dir(d) == d {
			return program
		}
	}
}

// Seconds is a duration as every step prints one: 12.3 s.
func Seconds(d time.Duration) string { return fmt.Sprintf("%.1f s", d.Seconds()) }

// Env is the environment with env's variables set over it.
func Env(env map[string]string) []string {
	out := os.Environ()
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// Run runs program with args in dir (by default here) with the terminal's output, and fails if it does. A run of a
// second or more says how long it took.
func Run(dir string, env map[string]string, program string, args ...string) error {
	where := ""
	if dir != "" && dir != "." {
		where = "   (in " + dir + ")"
	}
	fmt.Printf("$ %s %s%s\n", program, strings.Join(args, " "), where)
	start := time.Now()
	cmd := exec.Command(Bin(program, dir), args...)
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = dir, Env(env), os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	took := time.Since(start)
	if err != nil {
		return fmt.Errorf("%s %s failed (%v) after %s", program, strings.Join(args, " "), err, Seconds(took))
	}
	if took >= time.Second {
		fmt.Printf("took %s: %s\n", Seconds(took), short(program, args))
	}
	return nil
}

// Output runs program and returns its standard output; its error carries what it wrote to standard error.
func Output(dir string, env map[string]string, program string, args ...string) (string, error) {
	var out, errOut bytes.Buffer
	cmd := exec.Command(Bin(program, dir), args...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, Env(env), &out, &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s failed (%v): %s", program, strings.Join(args, " "), err, errOut.String())
	}
	return out.String(), nil
}

// Quietly runs program with its output kept, not shown: for what runs beside others. Its error carries that output.
func Quietly(dir string, env map[string]string, program string, args ...string) (time.Duration, error) {
	start := time.Now()
	cmd := exec.Command(Bin(program, dir), args...)
	cmd.Dir, cmd.Env = dir, Env(env)
	out, err := cmd.CombinedOutput()
	took := time.Since(start)
	if err != nil {
		return took, fmt.Errorf("%s %s failed (%v) after %s:\n%s", program, strings.Join(args, " "), err, Seconds(took), out)
	}
	return took, nil
}

func short(program string, args []string) string {
	line := program + " " + strings.Join(args, " ")
	if len(line) > 100 {
		return line[:97] + "..."
	}
	return line
}

// Cache is a folder in charter's own cache (the user's cache folder: ~/Library/Caches/charter on a Mac,
// ~/.cache/charter on Linux), made if it is not there: what a step may reuse when it did the same work before.
func Cache(parts ...string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(append([]string{base, "charter"}, parts...)...)
	return dir, os.MkdirAll(dir, 0o755)
}

// FreePort is a port nothing listens on.
func FreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// FreePorts are n ports nothing listens on, all different.
func FreePorts(n int) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for len(out) < n {
		p, err := FreePort()
		if err != nil {
			return nil, err
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out, nil
}

// Server is a program that serves until it is stopped.
type Server struct {
	Command []string          // the program and its arguments
	Dir     string            // where it runs; by default here
	Env     map[string]string // more environment
	Ready   string            // a URL that answers 2xx once it is up
}

// With starts s, waits until its Ready URL answers (two minutes at most), runs body, and stops s and what it
// started, whether body passed or failed.
func With(s Server, body func() error) error {
	fmt.Printf("$ %s   (started%s)\n", strings.Join(s.Command, " "), map[bool]string{true: " in " + s.Dir, false: ""}[s.Dir != ""])
	start := time.Now()
	cmd := exec.Command(Bin(s.Command[0], s.Dir), s.Command[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = s.Dir, Env(s.Env), os.Stdout, os.Stderr
	group(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait(); close(exited) }()
	defer stop(cmd, exited)
	if err := answers(s.Ready, exited); err != nil {
		return fmt.Errorf("%s: %w", s.Command[0], err)
	}
	fmt.Printf("took %s: %s answering at %s\n", Seconds(time.Since(start)), s.Command[0], s.Ready)
	return body()
}

func answers(url string, exited <-chan error) error {
	client := &http.Client{Timeout: 2 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for {
		select {
		case err := <-exited:
			return fmt.Errorf("stopped (%v) before %s answered", err, url)
		case <-ctx.Done():
			return errors.New(url + " did not answer in two minutes")
		default:
		}
		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			if resp.StatusCode/100 == 2 {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func stop(cmd *exec.Cmd, exited <-chan error) {
	kill(cmd)
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}
}
