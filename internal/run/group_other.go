//go:build !unix

package run

import (
	"fmt"
	"os/exec"
)

func group(*exec.Cmd) {}

// kill stops cmd and what it started (taskkill's tree).
func kill(cmd *exec.Cmd) {
	if cmd.Process != nil {
		exec.Command("taskkill", "/pid", fmt.Sprint(cmd.Process.Pid), "/T", "/F").Run()
	}
}
