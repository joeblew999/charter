// Package self runs this binary's own steps as commands of their own (go tool charter deploy …), for steps made of
// others: their flags, output and exit codes stay exactly theirs.
package self

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Output runs this binary with args, its stderr on ours, and returns its stdout, trimmed. A step that exits non-zero
// is an *exec.ExitError.
func Output(args ...string) (string, error) {
	bin, err := os.Executable()
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = &out, os.Stderr
	err = cmd.Run()
	return strings.TrimSpace(out.String()), err
}

// ExitCode is err's exit code: 0 for nil, the step's own for an exit, 1 otherwise.
func ExitCode(err error) int {
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	}
	return 1
}
