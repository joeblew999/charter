//go:build !unix

package run

import (
	"fmt"
	"os/exec"
	"strings"
)

func group(*exec.Cmd) {}

// kill stops cmd and what it started (taskkill's tree).
func kill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		exec.Command("taskkill", "/pid", fmt.Sprint(cmd.Process.Pid), "/T", "/F").Run()
	}
}

// KillGroup stops the process and what it started (taskkill's tree).
func KillGroup(pid int) { exec.Command("taskkill", "/pid", fmt.Sprint(pid), "/T", "/F").Run() }

// Alive reports whether the process is there.
func Alive(pid int) bool {
	out, _ := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
	return pid > 0 && strings.Contains(string(out), fmt.Sprint(pid))
}
