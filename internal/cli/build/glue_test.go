package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGlueForBothCompilers: build changes go.mjs by its text, for the standard compiler (TinyGo's clock workaround
// left out) and for -reuse=false. Both lines must be in worker/go.mjs, once each, or those builds fail.
func TestGlueForBothCompilers(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "worker", "go.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		`import "./tinygo-clock.mjs"; // before anything creates a Go runtime` + "\n",
		"const MAX_WAITING = 4;",
	} {
		if n := strings.Count(string(b), line); n != 1 {
			t.Errorf("worker/go.mjs has %q %d times, want once", line, n)
		}
	}
}
