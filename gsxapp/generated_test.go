package gsxapp_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestGeneratedIsCommitted: gsx binds a renderer or a component from another module to that module's generated Go
// (gsx's Package renderers guide: generate and publish the package before another module consumes it). So charter's
// own gsx packages commit their *.x.go, and this fails when one is not what its .gsx generates now.
func TestGeneratedIsCommitted(t *testing.T) {
	for _, dir := range []string{"../gsxapp", "../gsxintl", "../gsxseo"} {
		gen := exec.Command("go", "tool", "gsx", "generate")
		gen.Dir = dir
		gen.Env = append(gen.Environ(), "GOWORK=off")
		if out, err := gen.CombinedOutput(); err != nil {
			t.Fatalf("%s: gsx generate: %v\n%s", dir, err, out)
		}
		diff := exec.Command("git", "status", "--porcelain", "--", ".")
		diff.Dir = dir
		out, err := diff.Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.HasSuffix(line, ".x.go") {
				t.Errorf("%s: %s is not what its .gsx generates: commit it", dir, strings.TrimSpace(line))
			}
		}
	}
}
