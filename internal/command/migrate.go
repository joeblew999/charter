package command

import (
	"github.com/joeblew999/charter/internal/project"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Migrate runs charter migrate: an app written for an older charter, moved onto this one where that is mechanical:
// the import paths and the names that moved with them (the tables below), and the commands package.json's scripts
// name. What is left for a person, it lists.
func Migrate(args []string) {
	p := load()
	changed := 0
	check(filepath.WalkDir(p.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == "build" || d.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".go" && ext != ".gsx" && d.Name() != "gsx.toml" && d.Name() != "package.json" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)
		if d.Name() != "package.json" {
			text = moveImports(text)
		}
		if d.Name() == "package.json" {
			for old, now := range map[string]string{"charter new": "charter create", "charter sdk": "charter pack", "charter intl translate": "charter run translate", "charter live-test": "charter test --base"} {
				text = strings.ReplaceAll(text, old, now)
			}
		}
		if text != string(b) {
			changed++
			rel, _ := filepath.Rel(p.Root, path)
			say("moved %s", rel)
			return os.WriteFile(path, []byte(text), 0o644)
		}
		return nil
	}))
	if b, err := os.ReadFile(filepath.Join(p.Root, project.File)); err == nil {
		if text := moveSettings(string(b)); text != string(b) {
			check(os.WriteFile(filepath.Join(p.Root, project.File), []byte(text), 0o644))
			changed++
			say("moved %s: [check] and [pack]", project.File)
		}
	}
	say("✓ %d files moved", changed)
	for _, old := range []string{"intl.toml", "seo.toml", "mise.toml"} {
		if exists(filepath.Join(p.Root, old)) {
			say("· %s is still here: its settings are charter.toml's now ([intl], [seo]); mise is not needed", old)
		}
	}
	if n := uses(p.Root, "charter/mcp"); n != "" {
		say("· %s imports charter/mcp, which an app no longer imports: serve.Handler serves the MCP tools", n)
	}
	if changed > 0 {
		say("\nNext: charter check --fix && charter test")
	}
}

// moved are charter's import paths as its first releases had them, in folders: each is at the top now.
var moved = regexp.MustCompile(`github\.com/joeblew999/(?:charter/(?:cf|api|site)/|mise-intl/|mise-seo/)([a-z]+)`)

// merged are the packages that went into another (v0.29): the package, where it is now, and its exported names with
// what each is called there ("" when the name is the same). A name is only rewritten in a file that imported the
// package, and only where the package's name qualifies it, so a variable called hub or web is left alone.
var merged = []struct {
	old, now string
	names    map[string]string
}{
	{"site", "gsxapp", map[string]string{"App": "", "Head": "", "Entry": "", "HTML": ""}},
	{"web", "transport", map[string]string{"Allow": "", "GetOnly": "", "Origin": "", "Render": "", "WriteHTML": "", "Page": ""}},
	{"problem", "humaworkers", map[string]string{"Write": "WriteProblem"}},
	{"email", "bindings", map[string]string{"Send": "SendEmail", "Message": "EmailMessage", "ConsumeNonBlock": "ConsumeEmail", "Consumer": "EmailConsumer"}},
	{"hub", "live", map[string]string{"Hub": "", "DurableObject": "", "Memory": ""}},
	{"follow", "live", map[string]string{"Follow": "", "Newest": "", "Item": "", "Options": "", "Source": ""}},
	{"fragments", "live", map[string]string{"Data": "", "Stream": ""}},
	{"gsxseo", "gsxapp", map[string]string{"Head": "SearchHead"}},
}

// moveImports is a Go or gsx file's text with charter's packages imported from where they are now, and their names
// as they are called there.
func moveImports(text string) string {
	const charter = "github.com/joeblew999/charter/"
	text = moved.ReplaceAllString(text, charter+"$1")
	for _, m := range merged {
		old, now := `"`+charter+m.old+`"`, `"`+charter+m.now+`"`
		if !strings.Contains(text, old) {
			continue
		}
		// A file that imports a package of the same name from elsewhere (the app's own live) gets charter's under
		// another: charterlive.
		qualifier := m.now
		if regexp.MustCompile(`"[^"\n]*/` + m.now + `"`).MatchString(strings.ReplaceAll(text, now, "")) {
			qualifier = "charter" + m.now
			now = qualifier + " " + now
		}
		text = strings.ReplaceAll(text, old, now)
		for name, renamed := range m.names {
			if renamed == "" {
				renamed = name
			}
			text = regexp.MustCompile(`\b`+m.old+`\.`+name+`\b`).ReplaceAllString(text, qualifier+"."+renamed)
		}
	}
	// Two packages that are one now leave the same import twice: the second goes.
	seen := map[string]bool{}
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if t := strings.TrimSpace(line); strings.Contains(t, `"`+charter) && strings.HasSuffix(t, `"`) && !strings.Contains(t, "//") {
			if seen[t] {
				continue
			}
			seen[t] = true
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// uses is the first Go or gsx file under root that imports charter's package at path ("charter/mcp"), "" for none.
func uses(root, path string) string {
	found := ""
	filepath.WalkDir(root, func(file string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return filepath.SkipAll
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git") {
			return filepath.SkipDir
		}
		if ext := filepath.Ext(file); ext == ".go" || ext == ".gsx" {
			if b, err := os.ReadFile(file); err == nil && strings.Contains(string(b), `"github.com/joeblew999/`+path+`"`) {
				found, _ = filepath.Rel(root, file)
			}
		}
		return nil
	})
	return found
}

// moveSettings is a charter.toml with a block per command (v0.29): spec, spec_server and db, which were at the top,
// under [check], each with the comment above it; and [sdks] as [pack].
func moveSettings(text string) string {
	text = regexp.MustCompile(`(?m)^\[sdks\]$`).ReplaceAllString(text, "[pack]")
	lines := strings.Split(text, "\n")
	top := len(lines) // the top-level keys end at the first table
	for i, line := range lines {
		if strings.HasPrefix(line, "[") {
			top = i
			break
		}
	}
	var kept, moved []string
	key := regexp.MustCompile(`^(spec|spec_server|db)\s*=`)
	for i := 0; i < top; i++ {
		if !key.MatchString(lines[i]) {
			kept = append(kept, lines[i])
			continue
		}
		// The comment lines directly above a key are about it.
		n := len(kept)
		for n > 0 && strings.HasPrefix(kept[n-1], "#") && len(moved) == 0 {
			n--
		}
		moved = append(moved, kept[n:]...)
		kept = append(kept[:n], []string(nil)...)
		moved = append(moved, lines[i])
	}
	if len(moved) == 0 {
		return text
	}
	trim := func() {
		for len(kept) > 0 && kept[len(kept)-1] == "" {
			kept = kept[:len(kept)-1]
		}
	}
	trim()
	// Comment lines right above the first table are about that table: they stay above it.
	n := len(kept)
	for top < len(lines) && n > 0 && strings.HasPrefix(kept[n-1], "#") {
		n--
	}
	lead := append([]string(nil), kept[n:]...)
	if n > 0 && kept[n-1] != "" {
		lead = nil // a comment that runs on from the keys above it is theirs
	} else {
		kept = kept[:n]
		trim()
	}
	out := append(kept, "", "[check]")
	out = append(out, moved...)
	if top < len(lines) {
		out = append(out, "")
		out = append(out, lead...)
		out = append(out, lines[top:]...)
	} else {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}
