package scaffold

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joeblew999/charter/project"

	"github.com/BurntSushi/toml"

	"github.com/joeblew999/charter/internal/pins"
	"github.com/joeblew999/charter/internal/version"
)

// Report says every problem with charter.toml and the files charter writes into p's app, run from its root (charter
// check's first part), and whether there was none.
func Report(p *project.Project) bool {
	problems := Problems(p)
	for _, s := range problems {
		fmt.Println("✗", s)
	}
	if len(problems) == 0 {
		fmt.Printf("✓ %s and charter's files in this app\n", project.File)
	}
	return len(problems) == 0
}

// Problems is every problem with p's app, run from its root.
func Problems(p *project.Project) []string {
	var out []string
	for _, f := range owned() {
		b, err := os.ReadFile(f.path)
		switch {
		case os.IsNotExist(err) && strings.HasPrefix(f.path, ".github/"):
			// No CI of charter's: the app runs its own.
		case err != nil:
			out = append(out, fmt.Sprintf("%s: missing: %s", f.path, f.how))
		case !bytes.Equal(b, f.content):
			out = append(out, fmt.Sprintf("%s isn't this charter's: %s", f.path, f.how))
		case f.mode&0o111 != 0:
			if info, err := os.Stat(f.path); err == nil && info.Mode()&0o111 == 0 {
				out = append(out, fmt.Sprintf("%s isn't executable: %s", f.path, f.how))
			}
		}
	}
	gomod, _ := os.ReadFile("go.mod")
	// An app on a checkout of charter's library (a replace in go.mod, as charter's own Go examples are), or in one
	// (charter's TypeScript examples), runs that checkout's charter, not a release from mise.
	out = append(out, ownPins(!replaced.Match(gomod) && !inCharter())...)
	if s := skew(string(gomod), version.String(), version.Released()); s != "" {
		out = append(out, s)
	}
	if b, err := os.ReadFile(".gitignore"); err != nil || !strings.Contains("\n"+string(b), "\n/build/") {
		out = append(out, ".gitignore doesn't ignore /build/: charter init")
	}
	for _, m := range append([]string{p.Main}, p.Modules...) {
		if m == "" {
			continue
		}
		if _, err := os.Stat(m); err != nil {
			out = append(out, fmt.Sprintf("%s: module %s: %v", project.File, m, err))
		}
	}
	for _, d := range p.D1 {
		if d.Migrations == "" {
			continue
		}
		if files, _ := filepath.Glob(filepath.Join(d.Migrations, "*.sql")); len(files) == 0 {
			out = append(out, fmt.Sprintf("%s: [[d1]] %s: no *.sql in %s", project.File, d.Binding, d.Migrations))
		}
	}
	if date := workerdDate(); date != "" && p.CompatibilityDate > date {
		out = append(out, fmt.Sprintf("%s: compatibility_date %s is newer than the workerd on PATH (%s), which runs it locally", project.File, p.CompatibilityDate, date))
	}
	if p.CI.Build != "" || p.CI.Test != "" {
		if p.Repo == "" {
			out = append(out, fmt.Sprintf("%s: [ci] publishes each commit's build to repo, which isn't set", project.File))
		}
	}
	return out
}

// ownPins is each line of the app's own mise config that pins TinyGo or wasm-opt: mise would install a second copy
// beside the one charter pins (pins.ConfFile), and which the build ran would be chance. And, with requirePin, charter
// itself, which the app takes from mise (mise use github:joeblew999/charter): a problem when no config pins it.
func ownPins(requirePin bool) []string {
	var out []string
	pinned := false
	for _, file := range []string{"mise.toml", ".mise.toml", "mise.local.toml", ".config/mise.toml", ".config/mise/config.toml"} {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var config struct {
			Tools map[string]any `toml:"tools"`
		}
		if _, err := toml.Decode(string(b), &config); err != nil {
			out = append(out, fmt.Sprintf("%s: %v", file, err))
			continue
		}
		for name := range config.Tools {
			if pins.Charters(name) {
				out = append(out, fmt.Sprintf("%s pins %s: delete that line; charter pins it, at the version its TinyGo patch is for (%s: charter tasks)", file, name, pins.ConfFile))
			}
			pinned = pinned || name == CharterTool
		}
	}
	if requirePin && !pinned {
		out = append(out, fmt.Sprintf("mise.toml doesn't pin charter: mise use %s@<version>", CharterTool))
	}
	return out
}

// replaced matches a go.mod that replaces charter's library with a checkout of it.
var replaced = regexp.MustCompile(`(?m)^\s*(replace\s+)?github\.com/joeblew999/charter\s+=>`)

// charterModule matches charter's own go.mod.
var charterModule = regexp.MustCompile(`(?m)^module\s+github\.com/joeblew999/charter\s*$`)

// inCharter is whether the app, run from its root, is in a checkout of charter: the nearest go.mod above it is
// charter's own.
func inCharter() bool {
	dir, err := os.Getwd()
	if err != nil {
		return false
	}
	for d := filepath.Dir(dir); d != filepath.Dir(d); d = filepath.Dir(d) {
		if b, err := os.ReadFile(filepath.Join(d, "go.mod")); err == nil {
			return charterModule.Match(b)
		}
	}
	return false
}

// CharterTool is charter as mise has it: the binary of charter's GitHub release.
const CharterTool = "github:joeblew999/charter"

// skew is a problem when gomod requires charter's library at a version other than this charter's (binary), which
// builds the Worker from it: none when it requires none, replaces it (a checkout's), or this charter is no release.
func skew(gomod, binary string, released bool) string {
	if !released || replaced.MatchString(gomod) {
		return ""
	}
	m := regexp.MustCompile(`(?m)^\s*(?:require\s+)?github\.com/joeblew999/charter\s+(v\S+)`).FindStringSubmatch(gomod)
	if m == nil || m[1] == binary {
		return ""
	}
	return fmt.Sprintf("go.mod requires github.com/joeblew999/charter %s, but this charter is %s: the same version for both (mise use %s@%s, or go get github.com/joeblew999/charter@%s)", m[1], binary, CharterTool, strings.TrimPrefix(m[1], "v"), binary)
}
