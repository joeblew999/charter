package devloop

import (
	"fmt"
	"os/exec"
)

func own(*exec.Cmd) {}

// stop ends charter dev and everything it started (taskkill's tree).
func stop(cmd *exec.Cmd) {
	if cmd.Process != nil {
		exec.Command("taskkill", "/pid", fmt.Sprint(cmd.Process.Pid), "/T", "/F").Run()
		cmd.Wait()
	}
}
