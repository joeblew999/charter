//go:build !windows

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// ownGroup starts a server in a process group of its own, so that what it starts (go run's
// program, cf dev's workerd) stops with it.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// stopTree ends a process started with ownGroup and everything it started: asked first, then
// killed.
func stopTree(cmd *exec.Cmd, exited <-chan struct{}) {
	syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-exited
	}
}

// become runs a program in this process's place: its terminal, its signals, its exit code.
func become(path string, args, env []string) error {
	return syscall.Exec(path, append([]string{path}, args...), env)
}

// npmShim is the name of the file npm writes in node_modules/.bin for a package's program.
func npmShim(name string) string { return name }
