//go:build unix

package build

import "syscall"

// alive reports whether process pid is running.
func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }
