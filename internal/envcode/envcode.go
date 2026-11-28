// Package envcode is what a Worker is bound to, written as Go: an app's env/env.built.go, from the files that
// declare it. Cloudflare has this for TypeScript (wrangler types writes an Env from wrangler.jsonc); this is the
// same for Go.
//
//   - wrangler.jsonc: the databases, KV namespaces, buckets, queues, Durable Object namespaces, rate limits and
//     send_email bindings, and the variables (vars);
//   - .dev.vars.example and fnox.toml: the secrets, by name.
//
// Each becomes a value of its kind's type in charter's cloud package (cloud/names.go says why), under the name it
// has there: env.DB, env.WRITE_LIMIT, env.APP_NAME. So a binding that is not declared is a name that does not
// exist, and one of another kind does not fit: the compiler says both, on save.
//
// It is written wherever the app's other generated Go is (charter dev, build, check, test), never by hand, and is
// not committed, as the messages' Go is not. Lint finds the ways around it.
package envcode

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/tailscale/hujson"
)

// File is where the Go is written, in an app.
var File = filepath.Join("env", "env.built.go")

// Sources are the files it is written from: what charter dev watches.
var Sources = []string{"wrangler.jsonc", ".dev.vars.example", "fnox.toml"}

// bound is one name and how its value is made.
type bound struct{ name, value string }

// kind is the bindings of one kind, in the order they are written.
type kind struct {
	about string
	of    []bound
}

// goName is a name that is one in Go too, and exported: env.DB, env.WRITE_LIMIT.
var goName = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)

// read is what dir's files declare, by kind.
func read(dir string) ([]kind, error) {
	b, err := os.ReadFile(filepath.Join(dir, "wrangler.jsonc"))
	if err != nil {
		return nil, err
	}
	if b, err = hujson.Standardize(b); err != nil {
		return nil, fmt.Errorf("wrangler.jsonc: %w", err)
	}
	var w struct {
		Vars map[string]json.RawMessage `json:"vars"`
		D1   []struct {
			Binding string `json:"binding"`
		} `json:"d1_databases"`
		KV []struct {
			Binding string `json:"binding"`
		} `json:"kv_namespaces"`
		R2 []struct {
			Binding string `json:"binding"`
		} `json:"r2_buckets"`
		Queues struct {
			Producers []struct {
				Binding string `json:"binding"`
			} `json:"producers"`
		} `json:"queues"`
		Objects struct {
			Bindings []struct {
				Name string `json:"name"`
			} `json:"bindings"`
		} `json:"durable_objects"`
		Limits []struct {
			Name   string `json:"name"`
			Simple struct {
				Limit  int `json:"limit"`
				Period int `json:"period"`
			} `json:"simple"`
		} `json:"ratelimits"`
		Mail []struct {
			Name string `json:"name"`
		} `json:"send_email"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, fmt.Errorf("wrangler.jsonc: %w", err)
	}
	named := func(constructor string, names ...string) []bound {
		out := make([]bound, len(names))
		for i, name := range names {
			out[i] = bound{name, fmt.Sprintf("%s(%q)", constructor, name)}
		}
		return out
	}
	var d1, kv, r2, queues, objects, mail, vars []string
	for _, x := range w.D1 {
		d1 = append(d1, x.Binding)
	}
	for _, x := range w.KV {
		kv = append(kv, x.Binding)
	}
	for _, x := range w.R2 {
		r2 = append(r2, x.Binding)
	}
	for _, x := range w.Queues.Producers {
		queues = append(queues, x.Binding)
	}
	for _, x := range w.Objects.Bindings {
		objects = append(objects, x.Name)
	}
	for _, x := range w.Mail {
		mail = append(mail, x.Name)
	}
	for name := range w.Vars {
		vars = append(vars, name)
	}
	slices.Sort(vars)
	var limited []bound
	for _, x := range w.Limits {
		limited = append(limited, bound{x.Name, fmt.Sprintf("cloud.BoundRateLimit(%q, %d, %d)", x.Name, x.Simple.Limit, x.Simple.Period)})
	}
	secrets, err := secretNames(dir)
	if err != nil {
		return nil, err
	}
	// A secret that is also a variable (a local value in .dev.vars.example for one of wrangler.jsonc's vars) is the
	// variable.
	secrets = slices.DeleteFunc(secrets, func(name string) bool { return slices.Contains(vars, name) })
	kinds := []kind{
		{"D1 databases (wrangler.jsonc's d1_databases): Open is the database, for the queries.", named("cloud.BoundDatabase", d1...)},
		{"KV namespaces (kv_namespaces): Open is workers-go's *kv.Namespace.", named("cloud.BoundKV", kv...)},
		{"R2 buckets (r2_buckets): Open is workers-go's *r2.Bucket.", named("cloud.BoundBucket", r2...)},
		{"Queues the Worker sends to (queues.producers): Open is workers-go's *queues.Producer.", named("cloud.BoundQueue", queues...)},
		{"Durable Object namespaces (durable_objects.bindings): what a live.Feed's hub is in.", named("cloud.BoundObject", objects...)},
		{"Rate limits (ratelimits), with the calls and the seconds wrangler.jsonc says: Allow asks one.", limited},
		{"Email out (send_email): Send sends through one.", named("cloud.BoundMailer", mail...)},
		{"Variables (wrangler.jsonc's vars): Value is what one holds.", named("cloud.BoundVariable", vars...)},
		{"Secrets (.dev.vars.example, fnox.toml), by name: Value is what one holds on the Worker, never in the repository.", named("cloud.BoundVariable", secrets...)},
	}
	// A name is a name in Go, once.
	seen := map[string]bool{}
	for _, k := range kinds {
		for _, b := range k.of {
			switch {
			case !goName.MatchString(b.name):
				return nil, fmt.Errorf("%q is declared for the Worker (wrangler.jsonc, .dev.vars.example or fnox.toml), and is no name charter can write as Go (env.%s): start it with a capital letter, then letters, digits and _, as DB, WRITE_LIMIT and APP_NAME are", b.name, b.name)
			case seen[b.name]:
				return nil, fmt.Errorf("%s is declared twice for the Worker, as two kinds of thing (wrangler.jsonc): a binding's name is one thing", b.name)
			}
			seen[b.name] = true
		}
	}
	return kinds, nil
}

// secretNames are the secrets dir declares: the names of .dev.vars.example's lines, and of fnox.toml's secrets
// (its own and each profile's), sorted.
func secretNames(dir string) ([]string, error) {
	var names []string
	add := func(name string) {
		if name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, ".dev.vars.example")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if name, _, ok := strings.Cut(line, "="); ok && !strings.HasPrefix(line, "#") {
				add(strings.TrimSpace(name))
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "fnox.toml")); err == nil {
		var f struct {
			Secrets  map[string]any `toml:"secrets"`
			Profiles map[string]struct {
				Secrets map[string]any `toml:"secrets"`
			} `toml:"profiles"`
		}
		if err := toml.Unmarshal(b, &f); err != nil {
			return nil, fmt.Errorf("fnox.toml: %w", err)
		}
		for name := range f.Secrets {
			add(name)
		}
		for _, p := range f.Profiles {
			for name := range p.Secrets {
				add(name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

// Declared is the bindings of one kind an app declares: what the kind is, as one line, and their names.
type Declared struct {
	About string
	Names []string
}

// Read is what dir's files declare, by kind, in the order env/env.built.go has them: for what describes the
// Worker to a reader (the docs' page of it). A kind with nothing declared is left out.
func Read(dir string) ([]Declared, error) {
	kinds, err := read(dir)
	if err != nil {
		return nil, err
	}
	var out []Declared
	for _, k := range kinds {
		if len(k.of) == 0 {
			continue
		}
		d := Declared{About: k.about}
		for _, b := range k.of {
			d.Names = append(d.Names, b.name)
		}
		out = append(out, d)
	}
	return out, nil
}

// Go is dir's env/env.built.go, and "" for a Worker that declares nothing.
func Go(dir string) (string, error) {
	kinds, err := read(dir)
	if err != nil {
		return "", err
	}
	lines := []string{
		"// Code generated by charter from wrangler.jsonc, .dev.vars.example and fnox.toml. DO NOT EDIT.",
		"// Change those: charter dev, build, check and test write this again.",
		"",
		"// Package env is what the Worker is bound to, each under the name it is declared by, as a value of its kind:",
		"// env.DB.Open(), env.WRITE_LIMIT.Allow(key), env.APP_NAME.Value(). What is not declared is not here, so a",
		"// binding that is gone, or misspelt, does not compile.",
		"package env",
		"",
		`import "github.com/joeblew999/charter/cloud"`,
	}
	any := false
	for _, k := range kinds {
		if len(k.of) == 0 {
			continue
		}
		any = true
		lines = append(lines, "", "// "+k.about, "var (")
		for _, b := range k.of {
			lines = append(lines, "\t"+b.name+" = "+b.value)
		}
		lines = append(lines, ")")
	}
	if !any {
		return "", nil
	}
	out, err := format.Source([]byte(strings.Join(lines, "\n") + "\n"))
	return string(out), err
}

// Write writes dir's env/env.built.go, only when it differs (a file that changes is a build), and removes it for
// a Worker that declares nothing.
func Write(dir string) error {
	text, err := Go(dir)
	if err != nil {
		return err
	}
	file := filepath.Join(dir, File)
	if text == "" {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if b, err := os.ReadFile(file); err == nil && string(b) == text {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(text), 0o644)
}

// The packages whose functions take a binding's or a variable's name as text.
const (
	cloudPath   = "github.com/joeblew999/charter/cloud"
	workersPath = "github.com/syumai/workers-go/cloudflare"
)

// byName are the functions that take a name as text, by their package: each has a typed way in env.
var byName = map[string]map[string]string{
	workersPath:             {"Getenv": "env.NAME.Value()", "GetBinding": "the binding's value in env"},
	workersPath + "/kv":     {"NewNamespace": "env.NAME.Open()"},
	workersPath + "/r2":     {"NewBucket": "env.NAME.Open()"},
	workersPath + "/queues": {"NewProducer": "env.NAME.Open()"},
}

// Lint is every place in dir's Go, outside generated files, that names a binding or a variable as text:
// "built/mail.go:50: cloudflare.Getenv (env.NAME.Value())". Such a name compiles whatever it says. A call to
// cloud.Var is one (passing cloud.Var itself, to api.NewAuth, is not), and so is any cloud.Bound… function, which
// makes a binding's value from text and is the generated file's alone.
func Lint(dir string) ([]string, error) {
	var found []string
	files := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "build" || name == "dist" || name == "testdata" || name == "sdk") {
				return filepath.SkipDir
			}
			if path != dir {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".gen.go") || strings.HasSuffix(name, ".built.go") {
			return nil
		}
		file, err := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil // what does not parse is the compiler's to say
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		// The name each package is imported by, here.
		imported := map[string]string{}
		for _, imp := range file.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p != cloudPath && byName[p] == nil {
				continue
			}
			local := p[strings.LastIndex(p, "/")+1:]
			if imp.Name != nil {
				local = imp.Name.Name
			}
			imported[local] = p
		}
		if len(imported) == 0 {
			return nil
		}
		at := func(n ast.Node) int { return files.Position(n.Pos()).Line }
		of := func(e ast.Expr) (pkg, name string) {
			if sel, ok := e.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok {
					return imported[x.Name], sel.Sel.Name
				}
			}
			return "", ""
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				pkg, fn := of(n.Fun)
				if pkg == cloudPath && fn == "Var" {
					found = append(found, fmt.Sprintf("%s:%d: cloud.Var(…) (env.NAME.Value())", rel, at(n)))
				} else if instead := byName[pkg][fn]; instead != "" {
					found = append(found, fmt.Sprintf("%s:%d: %s.%s (%s)", rel, at(n), pkg[strings.LastIndex(pkg, "/")+1:], fn, instead))
				}
			case *ast.SelectorExpr:
				if pkg, fn := of(n); pkg == cloudPath && strings.HasPrefix(fn, "Bound") {
					found = append(found, fmt.Sprintf("%s:%d: cloud.%s (the binding's value in env)", rel, at(n), fn))
				}
			}
			return true
		})
		return nil
	})
	slices.Sort(found)
	return found, err
}
