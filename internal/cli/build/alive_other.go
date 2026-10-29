//go:build !unix

package build

import "os"

// alive reports whether process pid is running, as far as this platform can tell.
func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	p.Release()
	return true
}
