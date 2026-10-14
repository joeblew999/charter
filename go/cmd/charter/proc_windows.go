//go:build windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// ownGroup starts a server in a process group of its own, so that a Ctrl-C meant for this tool is
// not also the server's: the tool stops it.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// stopTree ends a process and everything it started (go run's program, cf dev's node and workerd).
// Windows has no signal to ask with: taskkill ends the tree by force.
func stopTree(cmd *exec.Cmd, exited <-chan struct{}) {
	exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		<-exited
	}
}

// become runs a program and ends with its exit code: Windows cannot replace a process.
func become(path string, args, env []string) error {
	cmd := exec.Command(path, args...)
	cmd.Env, cmd.Stdout, cmd.Stderr, cmd.Stdin = env, os.Stdout, os.Stderr, os.Stdin
	return cmd.Run()
}

// npmShim is the name of the file npm writes in node_modules/.bin for a package's program.
func npmShim(name string) string { return name + ".cmd" }
