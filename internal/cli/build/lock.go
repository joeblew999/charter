package build

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// lockFile keeps two builds out of build/ at once (a check while a dev loop rebuilds on a save): one's app.wasm beside
// the other's glue is a broken Worker. It lives outside build/, which a build deletes first.
const lockFile = ".build.lock"

// lock waits until no other build holds build/, then holds it; unlock lets it go. A lock whose builder is gone (killed)
// is taken over.
func lock() (unlock func(), err error) {
	waiting := false
	for {
		f, err := os.OpenFile(lockFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			fmt.Fprint(f, os.Getpid())
			f.Close()
			return func() { os.Remove(lockFile) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		b, _ := os.ReadFile(lockFile)
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if pid > 0 && !alive(pid) {
			os.Remove(lockFile)
			continue
		}
		if !waiting {
			fmt.Fprintf(os.Stderr, "· waiting for another build into build/ (pid %d)\n", pid)
			waiting = true
		}
		time.Sleep(200 * time.Millisecond)
	}
}
