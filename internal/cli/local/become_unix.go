//go:build unix

package local

import "syscall"

// becomeProcess replaces this process with bin (argv, env): workerd gets the terminal and the signals
// itself, and its exit is this command's.
func becomeProcess(bin string, argv, env []string) error {
	return syscall.Exec(bin, argv, env)
}
