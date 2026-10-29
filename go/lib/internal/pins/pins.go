// Package pins is the TinyGo and wasm-opt a Go Worker app builds with: mise.toml, which go tool charter tasks
// writes into an app (ConfFile) and go tool charter build checks the TinyGo on PATH against.
package pins

import (
	_ "embed"
	"strings"

	"github.com/BurntSushi/toml"
)

// File is mise.toml as an app gets it, at ConfFile.
//
//go:embed mise.toml
var File []byte

// ConfFile is where an app has File: mise reads it beside the app's own mise.toml, with no configuration.
const ConfFile = ".config/mise/conf.d/charter.toml"

// Tools is what File pins, by mise's name for each.
func Tools() map[string]string {
	var f struct {
		Tools map[string]string `toml:"tools"`
	}
	if _, err := toml.Decode(string(File), &f); err != nil {
		panic("pins: mise.toml: " + err.Error())
	}
	return f.Tools
}

// TinyGo is the TinyGo version this charter's runtime patches are for.
func TinyGo() string { return Tools()["tinygo"] }

// Charters reports whether a mise tool name is one of these tools, however it is written: tinygo, aqua:tinygo-org/tinygo,
// github:WebAssembly/binaryen, binaryen. An app pinning one itself would install a second copy beside charter's.
func Charters(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "tinygo") || strings.Contains(n, "binaryen")
}
