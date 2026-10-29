// Package pins is the TinyGo and wasm-opt a Go Worker app builds with: mise.toml, which charter tasks
// writes into an app (ConfFile) and charter build checks the TinyGo on PATH against; and the tools a repo charter keeps
// starts its mise.toml with (repo.toml, charter init).
package pins

import (
	_ "embed"
	"strings"
)

// File is mise.toml as an app gets it, at ConfFile.
//
//go:embed mise.toml
var File []byte

// ConfFile is where an app has File: mise reads it beside the app's own mise.toml, with no configuration.
const ConfFile = ".config/mise/conf.d/charter.toml"

// Repo is what a repo charter keeps starts its mise.toml with (charter init), by mise's name for each.
//
//go:embed repo.toml
var Repo []byte

// Tools is what File pins, by mise's name for each.
func Tools() map[string]string { return In(File) }

// In is what a mise config pins: its [tools] lines, name = "version", a name bare or quoted. Read with the standard
// library only, as the rest of the build step is.
func In(file []byte) map[string]string {
	tools := map[string]string{}
	in := false
	for _, line := range strings.Split(string(file), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			in = line == "[tools]"
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !in || !ok || strings.HasPrefix(line, "#") {
			continue
		}
		name, value = strings.Trim(strings.TrimSpace(name), `"`), strings.TrimSpace(value)
		if v, ok := strings.CutPrefix(value, `"`); ok {
			if end := strings.Index(v, `"`); end >= 0 {
				tools[name] = v[:end]
			}
		}
	}
	return tools
}

// TinyGo is the TinyGo version this charter's runtime patches are for.
func TinyGo() string { return Tools()["tinygo"] }

// Charters reports whether a mise tool name is one of these tools, however it is written: tinygo, aqua:tinygo-org/tinygo,
// github:WebAssembly/binaryen, binaryen. An app pinning one itself would install a second copy beside charter's.
func Charters(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "tinygo") || strings.Contains(n, "binaryen")
}
