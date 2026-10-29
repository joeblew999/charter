package pins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestPins(t *testing.T) {
	if TinyGo() == "" || Tools()["aqua:WebAssembly/binaryen"] == "" {
		t.Fatalf("mise.toml pins tinygo and binaryen: %v", Tools())
	}
	for name, want := range map[string]bool{"tinygo": true, "aqua:tinygo-org/tinygo": true, "github:WebAssembly/binaryen": true, "go": false, "workerd": false} {
		if Charters(name) != want {
			t.Errorf("Charters(%q) = %v, want %v", name, !want, want)
		}
	}
}

// TestSameAsCharters: an app builds with the TinyGo and wasm-opt charter itself builds and tests with, charter's own
// mise.toml (the repo's root, four levels up).
func TestSameAsCharters(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "mise.toml")
	b, err := os.ReadFile(root)
	if err != nil {
		t.Skipf("charter's mise.toml is not here (outside charter's repo): %v", err)
	}
	var charters struct {
		Tools map[string]any `toml:"tools"`
	}
	if _, err := toml.Decode(string(b), &charters); err != nil {
		t.Fatal(err)
	}
	for name, version := range Tools() {
		if got, _ := charters.Tools[name].(string); got != version {
			t.Errorf("%s: internal/pins/mise.toml says %q, charter's mise.toml %q: make them the same", name, version, got)
		}
	}
}
