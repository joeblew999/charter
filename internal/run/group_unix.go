//go:build unix

package run

import (
	"os/exec"
	"syscall"
)

// group starts cmd in a process group of its own, so stopping it stops what it started (go run's program, node's).
func group(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

// kill stops cmd's process group.
func kill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}
