package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// envPrefix is go/auth's EnvPrefix: the start of the variables an API's SDKs read.
func envPrefix(name string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(name))
}

// cloudflare calls Cloudflare's API for one account.
type cloudflare struct{ token, account string }

// cloudflareAPI is where Cloudflare's API is (a test's server, in tests).
var cloudflareAPI = "https://api.cloudflare.com/client/v4"

func newCloudflare() (cloudflare, error) {
	cf := cloudflare{secret("CLOUDFLARE_API_TOKEN"), secret("CLOUDFLARE_ACCOUNT_ID")}
	if cf.token == "" || cf.account == "" {
		return cf, errors.New("CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID are not set (in the environment, or in fnox)")
	}
	return cf, nil
}

var ids = regexp.MustCompile(`[0-9a-f-]{32,}`)

// call sends body (if any) to path under the account and decodes the result into out (if any).
// What it says on failure has no IDs in it.
func (cf cloudflare) call(method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, cloudflareAPI+"/accounts/"+cf.account+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cf.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var answer struct {
		Success bool            `json:"success"`
		Errors  json.RawMessage `json:"errors"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&answer); err != nil || !answer.Success {
		return fmt.Errorf("%s %s: HTTP %d %s", method, ids.ReplaceAllString(path, "<id>"), res.StatusCode, answer.Errors)
	}
	if out != nil {
		return json.Unmarshal(answer.Result, out)
	}
	return nil
}

// repoTasksFolder is the folder of charter's tasks/ with the tasks any repo takes, and repoTasks
// are those tasks (a test holds the list to the folder).
const repoTasksFolder = "repo"

// benchClient gives every request a limit, so a run can't hang on one.
var benchClient = &http.Client{Timeout: 30 * time.Second}

// A call is one operation made callable from the spec's own examples.
type call struct {
	method, path, trigger string
	body                  []byte
}

// example is a value for a parameter or a schema: its own example, or one built from its
// properties' examples (an object's required properties), following $ref.
func example(doc map[string]any, holder map[string]any) any {
	if v, ok := holder["example"]; ok {
		return v
	}
	schema, _ := holder["schema"].(map[string]any)
	for depth := 0; schema != nil && depth < 8; depth++ {
		ref, ok := schema["$ref"].(string)
		if !ok {
			break
		}
		schema, _ = dig(doc, strings.Split(strings.TrimPrefix(ref, "#/"), "/")...).(map[string]any)
	}
	if schema == nil {
		return nil
	}
	if examples := items(schema["examples"]); len(examples) > 0 {
		return examples[0]
	}
	for _, key := range []string{"example", "default"} {
		if v, ok := schema[key]; ok {
			return v
		}
	}
	if values := items(schema["enum"]); len(values) > 0 {
		return values[0]
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		object := map[string]any{}
		for _, name := range items(schema["required"]) {
			property, _ := properties[fmt.Sprint(name)].(map[string]any)
			value := example(doc, map[string]any{"schema": property})
			if value == nil {
				return nil
			}
			object[fmt.Sprint(name)] = value
		}
		return object
	}
	return nil
}

func dig(v any, keys ...string) any {
	for _, key := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[key]
	}
	return v
}

func items(v any) []any { l, _ := v.([]any); return l }

// secret is an environment variable, or the same name from fnox when the environment has none.
func secret(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	out, err := exec.Command("fnox", "get", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

type catalog struct {
	Owner string     `json:"owner"`
	Read  string     `json:"read"` // when, RFC 3339
	Repos []producer `json:"repos"`
}

type producer struct {
	Repo        string   `json:"repo"` // owner/name
	Description string   `json:"description,omitempty"`
	Topic       bool     `json:"topic"`             // has the topic charter
	Release     string   `json:"release,omitempty"` // the latest release's tag
	Binaries    []string `json:"binaries,omitempty"`
	Files       []string `json:"files,omitempty"` // the latest release's other files
	Specs       []string `json:"specs,omitempty"`
	GoModules   []string `json:"go_modules,omitempty"`
	Packages    []string `json:"packages,omitempty"` // npm
	Tasks       []string `json:"tasks,omitempty"`    // folders a mise.toml can include
	Deployed    []string `json:"deployed,omitempty"`
	Consumers   []pin    `json:"consumers"`
}

// pin is one place a repo says which version of another it uses.
type pin struct {
	Repo    string `json:"repo"`    // the consumer
	File    string `json:"file"`    // where, in the consumer
	Kind    string `json:"kind"`    // tool (mise [tools]), tasks (mise includes), go, npm
	Name    string `json:"name"`    // what it pins
	Version string `json:"version"` // as written
	Target  string `json:"-"`       // the producer, owner/name
}

var (
	// github.com/owner/name in a URL, a module path or a git remote.
	githubRepoRef = regexp.MustCompile(`github\.com[/:]([A-Za-z0-9_.-]+)/([A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*?)(?:\.git)?(?:[/#?"' ]|$)`)
	miseSection   = regexp.MustCompile(`^\[([^\]]+)\]\s*$`)
	miseTool      = regexp.MustCompile(`^"?((?:github|ubi|go):[^"=\s]+)"?\s*=\s*(.+)$`)
	miseInclude   = regexp.MustCompile(`git::https://github\.com/([^/]+)/([^/]+?)(?:\.git)?//([^?"]+)\?ref=([^"]+)`)
	quotedVersion = regexp.MustCompile(`^"([^"]*)"|version\s*=\s*"([^"]*)"`)
	goRequire     = regexp.MustCompile(`^(?:require\s+)?(github\.com/\S+)\s+(v\S+)(\s*//\s*indirect)?`)
	npmGitHub     = regexp.MustCompile(`^(?:github:|git\+https://github\.com/|https://github\.com/)([^/]+)/([^/#.]+)`)
	npmVersion    = regexp.MustCompile(`releases/download/([^/]+)/|#(.+)$`)
	apiURL        = regexp.MustCompile(`(?m)^API_URL\s*=.*?(https://[^'"\s}]+)`)
)

// docsUsing is the page in charter's folder that says how to use charter, written from the tool
// itself: the same in every repo, at the release its mise.toml pins, so what an agent is told matches
// the tool it has. AGENTS.md loads it with the rules.
func docsUsing() string {
	cell := strings.NewReplacer("|", `\|`, "\n", " ").Replace
	var page strings.Builder
	page.WriteString("---\ntitle: Using charter\nnav_order: 2\nparent: How this repo is kept\n---\n\n# Using charter\n\n" +
		"Written by `charter adopt` from the tool itself (the same page in every repo that uses it, at the release its `mise.toml` pins): don't edit it here, change the commands in `cmd/charter/` in [charter](https://github.com/joeblew999/charter).\n\n" +
		"charter is the tool this repo is kept with. mise installs the release `mise.toml` pins, and puts it on the path of every task.\n\n" +
		"| To | Run |\n|---|---|\n" +
		"| See what there is to run | `mise tasks` |\n" +
		"| Run a task | `mise run <task>` |\n" +
		"| Run the tool | `charter <command>`; from anywhere, `mise x " + toolRelease + " -- charter <command>` |\n" +
		"| Move to another release | `mise x " + toolRelease + "@X.Y.Z -- charter upgrade`: the pin and the includes together |\n" +
		"| Read more | [charter's docs](https://joeblew999.github.io/charter/); for machines, [llms.txt](https://joeblew999.github.io/charter/llms.txt) |\n\n" +
		"## The commands\n\nThose marked * work in any repo; the others in a project, a folder with a `mise.toml` beside a `fern/` folder.\n\n" +
		"| Command | What it does |\n|---|---|\n")
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		star := ""
		if anywhere[name] {
			star = " *"
		}
		usage := strings.TrimSpace(name + " " + commands[name].usage)
		fmt.Fprintf(&page, "| `%s`%s | %s |\n", cell(usage), star, cell(commands[name].help))
	}
	return page.String()
}

// githubRepo is what gh repo view --json says about a repo.
type githubRepo struct {
	NameWithOwner, Name, Description, HomepageURL string
	DefaultBranchRef                              struct{ Name string }
	RepositoryTopics                              []struct{ Name string }
}

var (
	mdLink    = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	mdHeading = regexp.MustCompile(`(?m)^#{1,6} +(.+?) *$`)
	miseRun   = regexp.MustCompile("mise run ([a-z][a-z0-9:-]*)")
	codeSpan  = regexp.MustCompile("`([^`\n]+)`")
	fenced    = regexp.MustCompile("(?s)```.*?```")
	gitTag    = regexp.MustCompile(`/v([0-9]|X\.)`) // go/v0.1.0 is a tag, not a path
	// A version of this tool's own releases written into a page: @v1.2.3 after a module of ours, or a
	// link to one tagged release.
	pinnedVersion = regexp.MustCompile(`(joeblew999/charter[A-Za-z0-9/_-]*@v[0-9]+\.[0-9]+\.[0-9]+|releases/(tag|download)/v[0-9][0-9A-Za-z.-]*)`)
	// A code span that is a path into the repo: one of its top-level entries, then a slash.
	repoPath = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./*<>{}-]*$`)
)

func gzipped(data []byte) (int, error) {
	var out bytes.Buffer
	w, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return 0, err
	}
	if _, err := w.Write(data); err != nil {
		return 0, err
	}
	if err := w.Close(); err != nil {
		return 0, err
	}
	return out.Len(), nil
}

// projectsBelow are the projects in the folders below dir, one or two levels down (examples/<name>).
func projectsBelow(dir string) ([]string, error) {
	var found []string
	for _, pattern := range []string{"*/mise.toml", "*/*/mise.toml"} {
		files, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if isProject(filepath.Dir(file)) {
				found = append(found, filepath.Dir(file))
			}
		}
	}
	sort.Strings(found)
	return found, nil
}

// tasksOf are the tasks a mise.toml defines itself and those it includes ([task_config] includes:
// charter's tasks/ folders). Mise also offers a folder the tasks of the mise.toml files above it,
// which is not what `each` asks. An include from GitHub is read by asking mise.
func tasksOf(file string) map[string]bool {
	tasks := map[string]bool{}
	content, err := os.ReadFile(file)
	if err != nil {
		return tasks
	}
	names, includes := tomlTasks(content)
	for _, name := range names {
		tasks[name] = true
	}
	for _, include := range includes {
		{
			if strings.Contains(include, "::") {
				for name := range miseTasks(filepath.Dir(file)) {
					tasks[name] = true
				}
				continue
			}
			// Relative in this repo's projects; absolute (the checkout's) in one made by new -from.
			folder := filepath.FromSlash(include)
			if !filepath.IsAbs(folder) {
				folder = filepath.Join(filepath.Dir(file), folder)
			}
			// An include is a folder of task files, or one task file by name (includes = ["tasks.toml"]:
			// a repo whose product is that file, which other repos include from git).
			files, _ := filepath.Glob(filepath.Join(folder, "*.toml"))
			if info, err := os.Stat(folder); err == nil && !info.IsDir() {
				files = []string{folder}
			}
			for _, included := range files {
				if content, err := os.ReadFile(included); err == nil {
					names, _ := tomlTasks(content)
					for _, name := range names {
						tasks[name] = true
					}
				}
			}
		}
	}
	return tasks
}

// miseTasks are the tasks mise offers in dir, or none if it cannot say.
func miseTasks(dir string) map[string]bool {
	tasks := map[string]bool{}
	out, err := output(dir, "mise", "tasks", "ls", "--json")
	if err != nil {
		return tasks
	}
	var listed []struct{ Name string }
	if json.Unmarshal([]byte(out), &listed) == nil {
		for _, task := range listed {
			tasks[task.Name] = true
		}
	}
	return tasks
}

// miseSections are the tables of a mise.toml that are not tasks: in an included task file, every
// other table is a task (["sdk:gen"]).
var miseSections = []string{"tools", "env", "settings", "task_config", "tasks", "plugins", "alias", "hooks", "vars", "redactions", "watch_files", "prepare"}

// tomlTasks are the tasks a mise.toml ([tasks.x]) or an included task file ([x]) defines, in the
// order written, and the includes it lists ([task_config] includes), read with a TOML library.
func tomlTasks(content []byte) (tasks, includes []string) {
	var raw map[string]any
	meta, err := toml.Decode(string(content), &raw)
	if err != nil {
		return nil, nil
	}
	for _, key := range meta.Keys() {
		switch {
		case len(key) == 2 && key[0] == "tasks":
			tasks = append(tasks, key[1])
		case len(key) == 1 && !slices.Contains(miseSections, key[0]):
			if _, table := raw[key[0]].(map[string]any); table {
				tasks = append(tasks, key[0])
			}
		}
	}
	if config, ok := raw["task_config"].(map[string]any); ok {
		list, _ := config["includes"].([]any)
		for _, include := range list {
			if text, ok := include.(string); ok {
				includes = append(includes, text)
			}
		}
	}
	return tasks, includes
}

const (
	repoURL    = "https://github.com/joeblew999/charter"
	repoModule = "github.com/joeblew999/charter"
	docsURL    = "https://joeblew999.github.io/charter/"
	// The tool as a module's package: what `go run` takes.
	toolPackage = repoModule + "/cmd/charter"
	// The tool as mise installs it: the release's binary for the system (GitHub's assets).
	toolRelease = "github:joeblew999/charter"

	// The example a new project is a copy of (Go; -lang ts: the TypeScript one), and its names in the
	// charter repo. Both name their Go SDK by the Go example's module.
	exampleDir    = "examples/notes-go"
	exampleModule = repoModule + "/" + exampleDir
	exampleWorker = "charter-notes-go"
	exampleDirTS  = "examples/notes-ts"
	// -empty: the same plumbing with one route (GET /api/hello) and no notes.
	startDir   = "examples/start-go"
	startDirTS = "examples/start-ts"
	// -empty -ui htmx: start-go's plumbing with server-rendered pages (gsx, htmx 4) in the same Worker.
	startDirHTMX = "examples/start-htmx"
	// -empty -ui datastar: the same with the pages made live by Datastar instead.
	startDirDatastar = "examples/start-datastar"
	// How the example's tasks run the tool: from the checkout they are in.
	exampleTool = "go run ../../cmd/charter"
	// And the tasks of a project made from a checkout: CHARTER, in its mise.toml, is that checkout.
	// (A template of mise's, which fills it in before any shell reads the line: $CHARTER is sh only.)
	checkoutTool = "go run {{env.CHARTER}}/cmd/charter"
	// Not copied: the committed Go SDK. It is generated from the specs, which carry the Worker's URL,
	// and it is a module under the example's path: `mise run sdk:publish` writes the project's own.
	exampleSDK = "sdk/go/"
)

// tsPackageURL is where a release's TypeScript library is: the package the release workflow attaches.
func tsPackageURL(version string) string {
	return repoURL + "/releases/download/" + version + "/charter-ts-" + strings.TrimPrefix(version, "v") + ".tgz"
}

// toolPin are the lines of a mise.toml's [tools] that pin the tool to a release: mise installs that
// release's binary, and it is on the path of every task.
func toolPin(version string) string {
	return "# The tool every task runs, the release's binary. `mise x " + toolRelease + "@X.Y.Z -- charter upgrade` moves it,\n# and the includes of charter's tasks with it.\n\"" + toolRelease + "\" = \"" + strings.TrimPrefix(version, "v") + "\"\n"
}

// tasksInclude is one of charter's tasks/ folders as a mise.toml includes it ([task_config]
// includes): from GitHub at the release, or the checkout's folder (forward slashes on every system:
// a backslash in a TOML string starts an escape).
func tasksInclude(folder, version, checkout string) string {
	if checkout != "" {
		return filepath.ToSlash(filepath.Join(checkout, "tasks", folder))
	}
	return "git::" + repoURL + ".git//tasks/" + folder + "?ref=" + version
}

// gitIncludes is the setting a mise.toml needs for the includes tasksInclude writes from GitHub
// (git::): mise counts task includes from a git source as experimental, and without it skips them
// with only a warning, so the repo has none of charter's tasks. A checkout's folders need nothing.
const gitIncludes = "[settings]\n# mise takes task includes from a git:: source only with this on; without it they are skipped,\n# with a warning, and none of charter's tasks are here.\nexperimental = true\n"

// withGitIncludes is a mise.toml with gitIncludes at its end, unless it says so already.
func withGitIncludes(text string) string {
	if strings.Contains(text, "\nexperimental = true") {
		return text
	}
	return strings.TrimRight(text, "\n") + "\n\n" + gitIncludes
}

// orCheckout names the tool's version, or says that it has none.
func orCheckout(version string) string {
	if version == "" {
		return "(not a release: built from a checkout)"
	}
	return version
}

// toolVersion is the release this tool was built from (go run ...cmd/charter@v0.3.0), or "" from a
// checkout.
func toolVersion() string {
	if built != "" && !strings.Contains(built, "SNAPSHOT") {
		return built // a binary from a GitHub Release
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || !strings.HasPrefix(info.Main.Version, "v") || strings.Contains(info.Main.Version, "-0.") {
		return "" // (devel), or a pseudo-version of an untagged commit
	}
	return info.Main.Version
}

func write(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// deleteScratch deletes a scratch Worker and its D1 database: perf's (<worker>-perf-*) or flow's
// (charter-flow-*). It refuses any other name.
func deleteScratch(worker string) error {
	if !strings.Contains(worker, "-perf-") && !strings.HasPrefix(worker, "charter-flow-") {
		return fmt.Errorf("%s is not a scratch Worker", worker)
	}
	if err := quiet(".", nil, npmBin("cf"), "workers", "delete", worker, "--force"); err != nil {
		return err
	}
	databases, err := d1Databases()
	if err != nil {
		return err
	}
	for _, d := range databases {
		if d.Name == worker+"-db" {
			if err := quiet(".", nil, npmBin("cf"), "d1", "delete", d.UUID, "--force"); err != nil {
				return err
			}
		}
	}
	fmt.Println("deleted", worker, "and its database")
	return nil
}

type d1Database struct{ Name, UUID string }

func d1Databases() ([]d1Database, error) {
	out, err := output(".", npmBin("cf"), "d1", "list", "--per-page", "100")
	if err != nil {
		return nil, err
	}
	var databases []d1Database
	if err := json.Unmarshal([]byte(out), &databases); err != nil {
		return nil, fmt.Errorf("cf d1 list: %w", err)
	}
	return databases, nil
}

const (
	// markLines is how far down a file its mark may be: under front matter and a title, in a page.
	markLines   = 12
	charterRepo = "github.com/joeblew999/charter"
)

var charterMark = regexp.MustCompile("(Written|Started) by `charter ([a-z-]+)`")

// markOf reads a file's mark: whether it is charter's to rewrite (or the repo's since charter
// started it), and the command that wrote it. No mark, or one that does not say whose the file is:
// false.
func markOf(content []byte) (charters bool, command string, ok bool) {
	lines := bytes.SplitN(content, []byte("\n"), markLines+1)
	if len(lines) > markLines {
		lines = lines[:markLines]
	}
	head := string(bytes.Join(lines, []byte("\n")))
	m := charterMark.FindStringSubmatch(head)
	switch {
	case m == nil:
		return false, "", false
	case m[1] == "Written" && strings.Contains(head, "don't edit"):
		return true, m[2], true
	case m[1] == "Started" && strings.Contains(head, "yours to edit"):
		return false, m[2], true
	}
	return false, "", false
}

// startedBy is a file that a command (new, adopt) writes once, with the mark that says so: a
// comment at its top, inside the front matter of a page that has some.
func startedBy(command, path, content string) string {
	mark := "Started by `charter " + command + "` (" + charterRepo + "): yours to edit."
	switch {
	case strings.HasSuffix(path, ".md") && strings.HasPrefix(content, "---\n"):
		return "---\n# " + mark + "\n" + strings.TrimPrefix(content, "---\n")
	case strings.HasSuffix(path, ".md"):
		return "<!-- " + mark + " -->\n\n" + content
	}
	return "# " + mark + "\n" + content
}

// distName starts the name of everything a project ships: the project's folder name, so that the
// files of several projects can sit on one GitHub Release.
func distName() (string, error) {
	dir, err := os.Getwd()
	return filepath.Base(dir), err
}

const dist = "dist"

// distDir makes dist/, which ignores itself: no repo that uses this tool has to list it in .gitignore.
func distDir() error {
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dist, ".gitignore"), []byte("*\n"), 0o644)
}

// Go module versions are semantic versions; anything after a hyphen is a pre-release.
var version = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`)

// built is the release a downloaded binary was made from: GoReleaser sets it (.goreleaser.yaml).
var built string

// A target of the CLI: the name a file on a release carries (<os>-<arch>, as Go and mise name them)
// and the Rust target it is built for. Linux is musl, a static binary that runs on any distribution
// (and the generated Cargo.toml then uses rustls, not the system's OpenSSL). Windows is the MinGW ABI,
// which zig links for both arches from any machine.
type cliTarget struct{ os, arch, rust string }

func (t cliTarget) String() string { return t.os + "-" + t.arch }

// cliFile is the CLI's name on a release: after the project, not the binary, as two projects' CLIs
// can have the same binary name.
func cliFile(name string, t cliTarget) string {
	file := name + "-cli-" + t.String()
	if t.os == "windows" {
		file += ".exe"
	}
	return file
}

// releaseTagEnv carries the tag from `release` to the tasks it runs (dist, release:publish,
// release:tags), as GitHub's variables carry it in a workflow on a tag.
const releaseTagEnv = "CHARTER_RELEASE_TAG"

// releaseTag is the version to release: -tag, or the tag `release` or a GitHub workflow was started
// for. None means a dry run.
func releaseTag(name string, args []string) (tag string, rest []string, err error) {
	rest = flags(name, args, func(f *flag.FlagSet) {
		f.StringVar(&tag, "tag", "", "the version tag (default: the tag being released, or the tag the workflow runs on; none: a dry run)")
	})
	if tag == "" {
		tag = os.Getenv(releaseTagEnv)
	}
	if tag == "" && os.Getenv("GITHUB_REF_TYPE") == "tag" {
		tag = os.Getenv("GITHUB_REF_NAME")
	}
	if tag != "" && !version.MatchString(tag) {
		return "", nil, fmt.Errorf("%q is not a version tag: it must look like v1.2.3 or v1.2.3-rc.1", tag)
	}
	return tag, rest, nil
}

// distFiles are the files in dist/ a release takes: not the ignore file distDir writes, not folders.
func distFiles() ([]string, error) {
	found, err := filepath.Glob(filepath.Join(dist, "[^.]*"))
	if err != nil {
		return nil, err
	}
	var files []string
	for _, file := range found {
		if info, err := os.Stat(file); err == nil && info.Mode().IsRegular() {
			files = append(files, file)
		}
	}
	return files, nil
}

// ensureRelease makes the tag's GitHub Release if there is none, with the notes of releaseNotes;
// with update, it also sets the notes and pre-release of one that is there (a workflow on the tag
// may have made it first). A tag with a hyphen (v1.2.0-rc.1) is always a pre-release.
func ensureRelease(tag string, prerelease bool, footer string, update bool) error {
	prerelease = prerelease || strings.Contains(tag, "-")
	exists := exec.Command("gh", "release", "view", tag).Run() == nil
	if exists && !update {
		return nil
	}
	notes, err := releaseNotes(tag, footer)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp("", "release-notes-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(notes); err != nil {
		return err
	}
	file.Close()
	if exists {
		return quiet(".", nil, "gh", "release", "edit", tag, "--title", tag, "--notes-file", file.Name(), fmt.Sprintf("--prerelease=%t", prerelease))
	}
	create := []string{"release", "create", tag, "--verify-tag", "--title", tag, "--notes-file", file.Name()}
	if prerelease {
		create = append(create, "--prerelease")
	}
	if out, err := exec.Command("gh", create...).CombinedOutput(); err != nil && exec.Command("gh", "release", "view", tag).Run() != nil {
		return fmt.Errorf("gh release create %s: %s", tag, strings.TrimSpace(string(out)))
	}
	return nil
}

// releaseNotes are the subjects of the commits since the version tag before target (all of them
// for the first), and the footer: text, or a file's.
func releaseNotes(target, footer string) (string, error) {
	logRange, heading := target, "## Changes"
	if before, err := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*", target+"^").Output(); err == nil {
		previous := strings.TrimSpace(string(before))
		logRange, heading = previous+".."+target, "## Changes since "+previous
	}
	subjects, err := exec.Command("git", "log", "--no-merges", "--format=- %s", logRange).Output()
	if err != nil {
		return "", fmt.Errorf("git log %s: %w", logRange, err)
	}
	notes := heading + "\n\n" + strings.TrimSpace(string(subjects)) + "\n"
	if footer != "" {
		if content, err := os.ReadFile(footer); err == nil {
			footer = string(content)
		}
		notes += "\n" + strings.TrimSpace(footer) + "\n"
	}
	return notes, nil
}

// An asset of a GitHub Release, with the SHA-256 GitHub computed when it was uploaded.
type asset struct{ Name, Digest string }

// releaseAssets are what the tag's Release carries; none if there is no Release.
func releaseAssets(tag string) []asset {
	out, err := exec.Command("gh", "api", "repos/{owner}/{repo}/releases/tags/"+tag).Output()
	if err != nil {
		return nil
	}
	var release struct{ Assets []asset }
	if json.Unmarshal(out, &release) != nil {
		return nil
	}
	return release.Assets
}

// charterToml is charter.toml, read with a TOML library.
type charterToml struct {
	Description string   `toml:"description"` // the repo's description on GitHub
	Topics      []string `toml:"topics"`      // the repo's topics on GitHub
	Homepage    string   `toml:"homepage"`    // default: the GitHub Pages URL
	Docs        string   `toml:"docs"`        // the docs folder, default "docs"
	Projects    []string `toml:"projects"`    // folders that are charter projects
	Renovate    bool     `toml:"renovate"`    // write renovate.json; default true
	Workflow    bool     `toml:"workflow"`    // in a repo that is not a project: write charter.yml; default true
}

var topicName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`)

// parseCharterToml reads charter.toml. A key it does not know is an error, not ignored.
func parseCharterToml(text string) (charterToml, error) {
	config := charterToml{Docs: "docs", Renovate: true, Workflow: true}
	meta, err := toml.Decode(text, &config)
	if err != nil {
		return config, fmt.Errorf("charter.toml: %w", err)
	}
	if unknown := meta.Undecoded(); len(unknown) > 0 {
		return config, fmt.Errorf("charter.toml: no key %q: description, topics, homepage, docs, projects, renovate, workflow", unknown[0].String())
	}
	config.Docs = filepath.ToSlash(filepath.Clean(config.Docs))
	if config.Description == "" {
		return config, errors.New(`charter.toml needs description = "..."`)
	}
	for _, topic := range config.Topics {
		if !topicName.MatchString(topic) {
			return config, fmt.Errorf("charter.toml: topic %q: GitHub takes lower-case letters, digits and hyphens, up to 50", topic)
		}
	}
	if !slices.Contains(config.Topics, charterTopic) {
		config.Topics = append(config.Topics, charterTopic)
	}
	if len(config.Topics) > 20 {
		return config, errors.New("charter.toml: GitHub takes up to 20 topics, charter's own among them")
	}
	if config.Docs != "docs" && config.Docs != "." {
		return config, errors.New(`charter.toml: docs is "docs" or "." (GitHub Pages serves a site from one of those)`)
	}
	return config, nil
}

// readCharterToml reads the charter.toml of the repo at root; without one, the defaults.
func readCharterToml(root string) (charterToml, error) {
	text, err := os.ReadFile(filepath.Join(root, "charter.toml"))
	if err != nil {
		return charterToml{Docs: "docs", Renovate: true, Workflow: true}, nil
	}
	return parseCharterToml(string(text))
}

// charterTopic is the topic every repo charter repo keeps has: charter catalog finds them by it.
const charterTopic = "charter"

// charterTomlFor is the charter.toml that charter new and charter adopt write.
func charterTomlFor(description string) string {
	return "# The repo as `charter repo` keeps it (mise run repo). Topics: lower-case words and hyphens, e.g. [\"docs\", \"command-line\"].\n" +
		"description = " + strconv.Quote(description) + "\ntopics = []\n"
}

// gh runs GitHub's command in dir and returns what it printed; on a failure the error says what gh
// said. Tests replace it.
var gh = func(dir string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stderr bytes.Buffer
	cmd.Dir, cmd.Stderr = dir, &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh %s: %v: %s", strings.Join(args[:min(2, len(args))], " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// npmBin is a program that an npm package of the project installed (package.json pins it): the
// file npm wrote for it in node_modules/.bin, which on Windows is <name>.cmd.
func npmBin(name string) string {
	shim := filepath.Join("node_modules", ".bin", npmShim(name))
	if abs, err := filepath.Abs(shim); err == nil {
		return abs
	}
	return shim
}

// npmInstalls reports whether an npm package of the project installs a program of this name:
// package-lock.json, which is committed, lists what every package puts in node_modules/.bin.
func npmInstalls(name string) bool {
	var lock struct {
		Packages map[string]struct{ Bin map[string]string }
	}
	raw, err := os.ReadFile("package-lock.json")
	if err != nil || json.Unmarshal(raw, &lock) != nil {
		return false
	}
	for path, pkg := range lock.Packages {
		if _, ok := pkg.Bin[name]; ok && strings.Count(path, "node_modules/") == 1 {
			return true
		}
	}
	return false
}

// program is where a command's first word is: the project's own install of it when an npm package
// brings one (cf, fern, tsc, vitest), otherwise the one on the path (go, node, mise). A program the
// project pins is never taken from the path: another version of it may be there.
func program(name string) (string, error) {
	if !strings.ContainsAny(name, `/\`) {
		if shim := npmBin(name); exists(shim) {
			return shim, nil
		}
		if npmInstalls(name) {
			return "", fmt.Errorf("no %s in node_modules/.bin: mise run setup installs the npm packages", name)
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("no %s on the path, and no npm package of the project installs one", name)
	}
	return path, nil
}

// settings checks -env values: each is NAME=VALUE.
func settings(env []string) error {
	for _, setting := range env {
		if name, _, ok := strings.Cut(setting, "="); !ok || name == "" {
			return fmt.Errorf("-env %s: want NAME=VALUE", setting)
		}
	}
	return nil
}

const (
	fernDir    = "fern"
	generators = fernDir + "/generators.yml"
	wiremock   = "ancestor=wiremock/wiremock:3.9.1"
)

// dockerWait is how long Docker has to answer: one that has gone to sleep with the Mac can say it is
// running and answer nothing, and then Fern would wait for it forever.
var dockerWait = 20 * time.Second

func docker() error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerWait)
	defer cancel()
	err := exec.CommandContext(ctx, "docker", "info").Run()
	switch {
	case ctx.Err() != nil:
		return fmt.Errorf("Docker doesn't answer (docker info, %s): restart it (OrbStack: orb stop && orb start; Docker Desktop: quit and open it)", dockerWait)
	case err != nil:
		return errors.New("Docker is not running: start it first")
	}
	return nil
}

// sdkStamp is where sdk-gen notes what a group was generated from (beside sdk/out/<group>, which
// Fern replaces whole).
func sdkStamp(group string) string { return filepath.Join("sdk/out", "."+group+sdkMadeFrom) }

// sdkOut is where Fern puts a group: the output path generators.yml names.
func sdkOut(group string) string {
	return filepath.Join("sdk/out", group)
}

// How a generated TypeScript SDK is typechecked: standard fetch, no settings of the project's own.
const sdkTSConfig = `{
  "//": "Written by charter sdk:check: how a generated TypeScript SDK is typechecked (bundler mode, default libs).",
  "compilerOptions": {
    "target": "es2022",
    "module": "preserve",
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "strict": true,
    "noEmit": true,
    "skipLibCheck": true
  },
  "include": [%q]
}
`

func sdkCheckDir(dir string) error {
	if exists(filepath.Join(dir, "go.mod")) {
		// A generated module is its own: no workspace file above it applies.
		off := []string{"GOWORK=off"}
		if err := quiet(dir, off, "go", "build", "./..."); err != nil {
			return err
		}
		fmt.Println("build ok")
		if err := quiet(dir, off, "go", "vet", "./..."); err != nil {
			return err
		}
		fmt.Println("vet ok")
		if compose := filepath.Join(dir, "wiremock", "docker-compose.test.yml"); exists(compose) {
			if err := quiet(".", nil, "docker", "compose", "-f", compose, "up", "-d", "--wait"); err != nil {
				return err
			}
			defer quiet(".", nil, "docker", "compose", "-f", compose, "down")
			// WireMock gets a random host port: tell the tests where it is.
			port, err := output(".", "docker", "compose", "-f", compose, "port", "wiremock", "8080")
			if err != nil {
				return err
			}
			off = append(off, "WIREMOCK_URL=http://"+port)
		}
		if err := quiet(dir, off, "go", "test", "./..."); err != nil {
			return err
		}
		fmt.Println("tests ok")
		return nil
	}
	for _, entry := range []string{"index.ts", "src/index.ts", "sdk/index.ts"} {
		entry = filepath.Join(dir, entry)
		if !exists(entry) {
			continue
		}
		// With the project's TypeScript (package.json), which resolves the SDK's imports from the
		// project's node_modules. Fern's SDKs are written for TypeScript 5: a project whose own code
		// is on a newer one installs 5.9 beside it as typescript-sdk ("npm:typescript@~5.9.3").
		config := filepath.Join(filepath.Dir(entry), "tsconfig.check.json")
		if err := os.WriteFile(config, fmt.Appendf(nil, sdkTSConfig, filepath.Base(entry)), 0o644); err != nil {
			return err
		}
		compiler := []string{npmBin("tsc")}
		if exists(tscForSDKs) {
			compiler = []string{"node", tscForSDKs}
		}
		if err := sh(".", compiler[0], append(compiler[1:], "-p", config)...); err != nil {
			return err
		}
		fmt.Printf("typecheck ok (%s)\n", entry)
		return nil
	}
	if entries, _ := os.ReadDir(dir); len(entries) == 0 {
		return fmt.Errorf("%s: nothing generated", dir)
	}
	// Another language: Fern generated it, and nothing here builds it.
	fmt.Printf("generated (%s): this tool builds and tests Go and TypeScript SDKs only\n", dir)
	return nil
}

// errNoCLI is what the CLI's commands say in a project without one.
var errNoCLI = errors.New("this project has no CLI (no cli group in fern/generators.yml): " + docsURL + "guides/sdks.html#add-the-cli")

// hasCLI reports whether the project ships a CLI: its generators.yml has the group cli.
func hasCLI() bool {
	groups, _ := sdkGroups(generators)
	return slices.Contains(groups, "cli")
}

// The tools that only the CLI needs, as a project's mise.toml pins them: Rust builds it, zig and
// cargo-zigbuild link it for the other systems. new -cli copies these lines from the notes example.
var cliPin = regexp.MustCompile(`(?m)^(rust|zig|"aqua:rust-cross/cargo-zigbuild") = "[^"]*"$`)

// cliPinsAgree fails unless the mise.toml in dir pins the CLI's tools exactly when generators.yml
// in dir has the cli group: the group is the switch, and the pins follow it.
func cliPinsAgree(dir string) error {
	tasks, err := os.ReadFile(filepath.Join(dir, "mise.toml"))
	if err != nil {
		return err
	}
	groups, err := sdkGroups(filepath.Join(dir, generators))
	if err != nil {
		return err
	}
	pins := len(cliPin.FindAll(tasks, -1))
	switch cli := slices.Contains(groups, "cli"); {
	case cli && pins != 3:
		return errors.New("fern/generators.yml has the cli group, but mise.toml does not pin rust, zig and \"aqua:rust-cross/cargo-zigbuild\" (the notes example's lines)")
	case !cli && pins != 0:
		return errors.New("mise.toml pins the CLI's tools (rust, zig, cargo-zigbuild), but fern/generators.yml has no cli group: remove them")
	}
	return nil
}

// sdkGroups are the groups a generators.yml defines, in the file's order.
func sdkGroups(file string) ([]string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var config struct {
		Groups yaml.Node `yaml:"groups"`
	}
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}
	var groups []string
	for i := 0; i+1 < len(config.Groups.Content); i += 2 { // a mapping: key, value, key, value
		groups = append(groups, config.Groups.Content[i].Value)
	}
	return groups, nil
}

// exe is the file name of a program this system built: on Windows it ends in .exe.
func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// cargoBin is the name of the first [[bin]] in a generated CLI's Cargo.toml.
func cargoBin(dir string) (string, error) {
	manifest, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		return "", err
	}
	_, bins, ok := strings.Cut(string(manifest), "[[bin]]")
	if m := regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`).FindStringSubmatch(bins); ok && m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("%s/Cargo.toml has no [[bin]] name", dir)
}

var rustVersion = regexp.MustCompile(`^rustc (\d+\.\d+\.\d+)`)

// cliBuildDir compiles every Rust dependency the first time: minutes at full CPU.
func cliBuildDir(dir string, linux bool) error {
	bin, err := cargoBin(dir)
	if err != nil {
		return err
	}
	build := []string{"build", "--release", "--bin", bin, "--no-default-features", "--features", "rustls"}
	if !linux {
		if err := sh(dir, "cargo", build...); err != nil {
			return err
		}
		fmt.Printf("built: %s/target/release/%s\n", dir, exe(bin))
		return nil
	}
	if err := docker(); err != nil {
		return err
	}
	// The same Rust as the native build: the one the project's mise.toml pins.
	have, err := output(".", "rustc", "--version")
	channel := rustVersion.FindStringSubmatch(have)
	if err != nil || channel == nil {
		return errors.New("no rustc: the project's mise.toml pins Rust (mise install)")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	// Named volumes cache crates between runs; target-linux keeps it apart from a native build.
	run := append([]string{"run", "--rm", "-v", abs + ":/src", "-w", "/src", "-v", "charter-cargo-registry:/usr/local/cargo/registry",
		"-e", "CARGO_TARGET_DIR=/src/target-linux", "rust:" + channel[1], "cargo"}, build...)
	if err := sh(".", "docker", run...); err != nil {
		return err
	}
	fmt.Printf("built: %s/target-linux/release/%s (Linux)\n", dir, bin)
	return nil
}

// sdkMadeFrom is a file in the committed copy that records which specs it was generated from, so
// that a stale copy shows without generating it again: sdk-publish -check -quick.
const sdkMadeFrom = ".made-from"

// sdkSourceHash is a hash of what an SDK is generated from, the project's Fern folder: the specs,
// generators.yml, an overlay, Fern's settings. Not docs.yml: that is the reference site's.
func sdkSourceHash() (string, error) {
	entries, err := os.ReadDir(fernDir)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, entry := range entries { // ReadDir sorts by name
		if !entry.Type().IsRegular() || entry.Name() == "docs.yml" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(fernDir, entry.Name()))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(hash, "%s %d\n", entry.Name(), len(content))
		hash.Write(content)
	}
	return fmt.Sprintf("sha256:%x %s", hash.Sum(nil), fernDir), nil
}

// The Worker's secrets: the names WORKER_SECRETS (in mise.toml) lists, e.g. READ_TOKEN WRITE_TOKEN.
func workerSecretNames() []string { return strings.Fields(os.Getenv("WORKER_SECRETS")) }

// The Worker's optional secrets (WORKER_OPTIONAL_SECRETS), e.g. ACCESS_AUD: set when the environment
// has them, left alone when it has not.
func optionalSecretNames() []string { return strings.Fields(os.Getenv("WORKER_OPTIONAL_SECRETS")) }

// secretsFile writes the secrets, by name, to a file only this user can read, for cf deploy
// --secrets-file. The caller removes it. "" when there are none.
func secretsFile(values map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	file, err := os.CreateTemp("", "charter-secrets-*.json") // mode 0600
	if err != nil {
		return "", err
	}
	if err := json.NewEncoder(file).Encode(values); err != nil {
		file.Close()
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), file.Close()
}

type list []string

func (l *list) String() string { return strings.Join(*l, "; ") }

func (l *list) Set(v string) error { *l = append(*l, v); return nil }

// server starts a program (start: its words; env: variables for it) in the current folder and waits
// until url answers. stop ends it and everything it started.
func server(start, env []string, url string) (stop func(), err error) {
	if _, err := http.Get(url); err == nil {
		return nil, fmt.Errorf("%s already answers: something else is running there", url)
	}
	path, err := program(start[0])
	if err != nil {
		return nil, err
	}
	log, err := os.CreateTemp("", "charter-server-*.log")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, start[1:]...)
	cmd.Stdout, cmd.Stderr, cmd.Env = log, log, append(os.Environ(), env...)
	ownGroup(cmd) // so what it starts stops with it
	forget := func() { log.Close(); os.Remove(log.Name()) }
	if err := cmd.Start(); err != nil {
		forget()
		return nil, err
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }()
	stopped := false
	stop = func() {
		if stopped {
			return
		}
		stopped = true
		stopTree(cmd, exited)
		forget()
	}
	showLog := func() {
		if out, err := os.ReadFile(log.Name()); err == nil {
			os.Stderr.Write(out)
		}
	}
	said := strings.Join(start, " ")
	for deadline := time.Now().Add(90 * time.Second); time.Now().Before(deadline); time.Sleep(500 * time.Millisecond) {
		select {
		case <-exited:
			showLog()
			forget()
			return nil, fmt.Errorf("the server stopped before %s answered: %s", url, said)
		default:
		}
		if res, err := http.Get(url); err == nil {
			res.Body.Close()
			return stop, nil
		}
	}
	showLog()
	stop()
	return nil, fmt.Errorf("%s did not answer within 90 s: %s", url, said)
}

// freePorts asks for n different ones: it holds them all before letting any go.
func freePorts(n int) ([]string, error) {
	var ports []string
	for range n {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			return nil, err
		}
		defer listener.Close()
		ports = append(ports, strconv.Itoa(listener.Addr().(*net.TCPAddr).Port))
	}
	return ports, nil
}

// workerFlags are what both migrate commands take. The Worker is the project's (the first label of
// API_URL's host) unless -worker names another: a scratch Worker of charter perf:try.
func workerFlags(name string, args []string, port *string) (string, error) {
	var worker string
	flags(name, args, func(f *flag.FlagSet) {
		f.StringVar(&worker, "worker", "", "the Worker's name (default: the project's, from API_URL)")
		if port != nil {
			f.StringVar(port, "port", "", "the dev server's port (default: API_PORT)")
		}
	})
	if worker != "" {
		return worker, nil
	}
	_, worker, err := deployed()
	return worker, err
}

func migrations() ([]string, error) {
	files, err := filepath.Glob("migrations/*.sql")
	sort.Strings(files)
	if err == nil && len(files) == 0 {
		err = errors.New("no migrations/*.sql")
	}
	return files, err
}

func size(args []string) error {
	var max int
	rest := flags("size", args, func(f *flag.FlagSet) { f.IntVar(&max, "max", 0, "the limit in bytes, gzipped") })
	if len(rest) != 1 || max == 0 {
		return errors.New("size needs -max <bytes> <file>")
	}
	raw, err := os.ReadFile(rest[0])
	if err != nil {
		return err
	}
	gz, err := gzipped(raw)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d B, %d B gzipped (limit %d)\n", rest[0], len(raw), gz, max)
	if gz > max {
		return fmt.Errorf("%s is over the limit", rest[0])
	}
	return nil
}

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func anyMap[V any](m map[string]V) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// miseTask is a task as `mise tasks ls --json` gives it.
type miseTask struct {
	Name, Description, Source string
	Hide, Global              bool
}

var errNoTasks = errors.New("docs-tasks: no tasks to list (hidden tasks and those of a mise config outside the repo are left out)")

// docsTasksPage is the page docs-tasks prints for the repo at root. charter docs calls it itself
// for a generated page whose command is `charter docs-tasks`, so the page is written by the tool
// that checks it, whatever charter is on the path. A repo with no mise.toml yet has a page that
// says so: every repo charter keeps has this page from its first day.
func docsTasksPage(root string, args []string) (string, error) {
	var only, not, title string
	flags("docs-tasks", args, func(f *flag.FlagSet) {
		f.StringVar(&only, "only", "", "only the tasks from these sources, comma-separated: a source is named as the page's headings name it (mise.toml, tasks.toml, owner/repo//folder); a part of the name is enough")
		f.StringVar(&not, "not", "", "leave out the tasks from these sources, named the same way")
		f.StringVar(&title, "title", "Tasks", "the page's title")
	})
	// mise names each task's file by its full path: the repo's root must be one too, or the repo's
	// own tasks look like another repo's (charter docs passes the root as it was given: ".").
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	config, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		return "# " + title + "\n\nThis repo has no `mise.toml` at its root yet. Its tasks are listed here once it has one.\n", nil
	}
	// Listing tasks runs nothing, so mise may read this repo's config before the developer has
	// trusted it (a repo just adopted, a fresh clone on a CI runner).
	mise := func(args ...string) (string, error) {
		cmd := exec.Command("mise", args...)
		cmd.Dir, cmd.Stderr = root, os.Stderr
		cmd.Env = append(os.Environ(), "MISE_TRUSTED_CONFIG_PATHS="+root)
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
	listed, err := mise("tasks", "ls", "--json")
	if err != nil {
		return "", errors.New("docs-tasks: mise tasks ls --json failed")
	}
	var tasks []miseTask
	if err := json.Unmarshal([]byte(listed), &tasks); err != nil {
		return "", fmt.Errorf("docs-tasks: what mise tasks ls --json printed: %w", err)
	}
	details, err := mise("generate", "task-docs", "--style", "detailed")
	if err != nil {
		return "", errors.New("docs-tasks: mise generate task-docs failed")
	}
	page, err := tasksPage(title, root, includesOf(string(config)), tasks, details, commaList(only), commaList(not), os.ReadFile)
	if errors.Is(err, errNoTasks) {
		return "# " + title + "\n\nThis repo has no mise task to list yet.\n", nil
	}
	return page, err
}

// commaList is a comma-separated flag's values.
func commaList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// includesOf are the entries of a mise.toml's [task_config] includes, in the order written.
func includesOf(config string) []string {
	_, includes := tomlTasks([]byte(config))
	return includes
}

var (
	// An include from a git repo: git::https://github.com/<owner>/<repo>.git//<path>?ref=<ref>
	gitInclude = regexp.MustCompile(`^git::https?://[^/]+/(.+?)(?:\.git)?//([^?]+)`)
	// Where mise keeps a git include it fetched: .../remote-git-tasks-cache/<hash>/<path in the repo>
	miseGitCache = regexp.MustCompile(`remote-git-tasks-cache/[^/]+/(.+)$`)
	// A task's block in what mise generate task-docs prints.
	taskBlock = regexp.MustCompile("(?m)^## `([^`]+)`")
)

// tasksPage is the page: its title, how to run a task, then for each source a table of its tasks
// and what each does, then every task with its usage, arguments and flags as mise documents them.
// tasks are mise's (the hidden ones and those of a config outside the repo are left out), details
// is what mise generate task-docs --style detailed printed, and read reads a task file for the
// order its tasks are written in.
func tasksPage(title, root string, includes []string, tasks []miseTask, details string, only, not []string, read func(string) ([]byte, error)) (string, error) {
	type group struct {
		name  string
		tasks []miseTask
	}
	// The sources, in order: the repo's own mise.toml, then its includes as listed.
	names := []string{"mise.toml"}
	for _, include := range includes {
		names = append(names, sourceName(include))
	}
	groups := map[string]*group{}
	rank := map[string]int{} // a task's place: its file among the group's, then its place in the file
	order := map[string][]string{}
	for _, task := range tasks {
		name, ok := sourceOf(root, includes, task.Source)
		if task.Hide || task.Global || !ok {
			continue
		}
		if groups[name] == nil {
			groups[name] = &group{name: name}
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
		groups[name].tasks = append(groups[name].tasks, task)
		if _, seen := order[task.Source]; !seen {
			order[task.Source] = nil
			if content, err := read(task.Source); err == nil {
				order[task.Source], _ = tomlTasks(content)
			}
		}
		at := slices.Index(order[task.Source], task.Name)
		if at < 0 {
			at = len(order[task.Source]) // a task that is a file of its own: after the written ones
		}
		rank[task.Name] = at
	}
	chosen := func(name string) bool {
		match := func(parts []string) bool {
			return slices.ContainsFunc(parts, func(part string) bool { return strings.Contains(name, part) })
		}
		return (len(only) == 0 || match(only)) && !match(not)
	}
	for _, part := range append(slices.Clone(only), not...) {
		if !slices.ContainsFunc(names, func(name string) bool { return groups[name] != nil && strings.Contains(name, part) }) {
			var have []string
			for _, name := range names {
				if groups[name] != nil {
					have = append(have, name)
				}
			}
			return "", fmt.Errorf("docs-tasks: no tasks come from a source named %q: the sources are %s", part, strings.Join(have, ", "))
		}
	}
	blocks := map[string]string{}
	at := taskBlock.FindAllStringSubmatchIndex(details, -1)
	for i, m := range at {
		end := len(details)
		if i+1 < len(at) {
			end = at[i+1][0]
		}
		blocks[details[m[2]:m[3]]] = details[m[1]:end]
	}

	var page, each strings.Builder
	page.WriteString("# " + title + "\n\nRun one with `mise run <task>`; its arguments and flags go after `--`. `mise tasks` lists them, and `mise run <task> --help` shows one.\n")
	count := 0
	for _, name := range names {
		g := groups[name]
		if g == nil || !chosen(name) {
			continue
		}
		sort.SliceStable(g.tasks, func(i, j int) bool {
			a, b := g.tasks[i], g.tasks[j]
			if a.Source != b.Source {
				return a.Source < b.Source
			}
			if rank[a.Name] != rank[b.Name] {
				return rank[a.Name] < rank[b.Name]
			}
			return a.Name < b.Name
		})
		page.WriteString("\n## From `" + name + "`\n\n| Task | What it does |\n|---|---|\n")
		for _, task := range g.tasks {
			count++
			page.WriteString("| [`" + task.Name + "`](#" + anchorOf("`"+task.Name+"`") + ") | " + cellText(task.Description) + " |\n")
			each.WriteString("\n### `" + task.Name + "`\n\n" + taskDetail(blocks[task.Name]) + "\n")
		}
	}
	if count == 0 {
		return "", errNoTasks
	}
	page.WriteString("\n## Each task\n" + each.String())
	// The site's renderer reads two curly braces, or one and a percent sign, as template code.
	return strings.NewReplacer("{{", "&#123;&#123;", "{%", "&#123;%").Replace(page.String()), nil
}

// sourceName is how the page names an include: a file or folder of the repo as it is written, a
// git include as owner/repo//folder (no ref: the page does not change with every release).
func sourceName(include string) string {
	if m := gitInclude.FindStringSubmatch(include); m != nil {
		return m[1] + "//" + strings.TrimSuffix(m[2], "/")
	}
	path := strings.TrimSuffix(filepath.ToSlash(include), "/")
	if filepath.IsAbs(include) {
		// A folder outside the repo (a checkout of charter): its last two names, not this machine's path.
		parts := strings.Split(path, "/")
		path = strings.Join(parts[max(0, len(parts)-2):], "/")
	}
	return path
}

// sourceOf is the source a task's file belongs to, by the name the page gives it: the include it is
// or is under, else its path in the repo. A file outside the repo that no include accounts for (a
// mise config above the repo, or the user's own) is not the repo's: false.
func sourceOf(root string, includes []string, file string) (string, bool) {
	slashed := filepath.ToSlash(file)
	inCache := miseGitCache.FindStringSubmatch(slashed)
	rel, err := filepath.Rel(root, file)
	inRepo := err == nil && !strings.HasPrefix(rel, "..") && inCache == nil
	rel = filepath.ToSlash(rel)
	for _, include := range includes {
		if m := gitInclude.FindStringSubmatch(include); m != nil {
			path := strings.TrimSuffix(m[2], "/")
			if inCache != nil && (inCache[1] == path || strings.HasPrefix(inCache[1], path+"/")) {
				return sourceName(include), true
			}
			continue
		}
		path := strings.TrimSuffix(filepath.ToSlash(include), "/")
		if filepath.IsAbs(include) && (slashed == path || strings.HasPrefix(slashed, path+"/")) {
			return sourceName(include), true
		}
		if inRepo && (rel == path || strings.HasPrefix(rel, path+"/")) {
			return path, true
		}
	}
	switch {
	case inRepo:
		return rel, true
	case inCache != nil:
		return inCache[1], true
	}
	return "", false
}

// anchorOf is the anchor GitHub and the docs site give a heading (as docs-lint's hasAnchor reads it).
func anchorOf(heading string) string {
	return strings.ReplaceAll(regexp.MustCompile("[^a-z0-9 _-]").ReplaceAllString(strings.ToLower(heading), ""), " ", "-")
}

// cellText is text as a table cell holds it: an angle bracket would start a tag, a bar end the
// cell, and two dashes become one long one.
func cellText(text string) string {
	return strings.NewReplacer("<", "&lt;", ">", "&gt;", "|", `\|`, "--", `\-\-`, "\n", " ").Replace(strings.TrimSpace(text))
}

// taskDetail is a task's block of mise's own docs under the page's heading for it: without the
// tasks it depends on (hidden steps are not something to run), and with its Arguments and Flags
// headings as bold lines, so the page's outline is its tasks.
func taskDetail(block string) string {
	var lines []string
	for _, line := range strings.Split(block, "\n") {
		switch {
		case strings.HasPrefix(line, "- Depends:"):
			continue
		case line == "### Arguments" || line == "### Flags":
			line = "**" + strings.TrimPrefix(line, "### ") + "**"
		}
		lines = append(lines, line)
	}
	text := regexp.MustCompile(`\n{3,}`).ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
	return strings.TrimSpace(text)
}

// libraryModule is the Go library a Go Worker is built on (go/ in the charter repo): its packages,
// and in worker/ the JavaScript that the packages transport and hub talk to.
const libraryModule = repoModule + "/go"

// tree makes to a copy of the folder from. With link, a file is a hard link to the one in from
// (the same file under a second name: no disk, and nothing may write into it) where the system
// allows one, which it does not across drives.
func tree(from, to string, link bool) error {
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			if points, err := os.Readlink(path); err == nil && os.Symlink(points, target) == nil {
				return nil
			}
		} else if link && os.Link(path, target) == nil {
			return nil
		}
		info, err := os.Stat(path) // what a link points at
		if err != nil {
			return err
		}
		if info.IsDir() {
			return tree(path, target, link)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

const repoWorkflow = "charter.yml"

// shipsCLI reports whether the repo in dir releases a CLI: its project (at the root) or one of the
// projects in its examples/ has the cli group in fern/generators.yml.
func shipsCLI(dir string) bool {
	files, _ := filepath.Glob(filepath.Join(dir, "examples", "*", generators))
	for _, file := range append([]string{filepath.Join(dir, generators)}, files...) {
		if groups, _ := sdkGroups(file); slices.Contains(groups, "cli") {
			return true
		}
	}
	return false
}
