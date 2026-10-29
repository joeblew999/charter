package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/project"
)

// charterJobs are charter's own jobs, the same in every repo: their workflows (charter-<job>.yml) are charter's.
var charterJobs = map[string]project.Job{
	"repo": {
		About: "What charter checks in any repo: charter's files and the workflows (init -check), the docs, the repo on GitHub as charter.toml says (repo -check), the upstream issues the code works around",
		On:    []string{"push", "pull_request"},
		Run:   []string{"charter init -check", "charter docs", "charter repo -check", "charter upstream"},
		// What the checks read with gh, and nothing they could change.
		Permissions: map[string]string{"contents": "read", "issues": "read", "pull-requests": "read", "pages": "read"},
		Timeout:     10,
	},
}

// Jobs are every job of the repo at root: charter's, and its own (charter.toml's [ci.jobs]).
func Jobs(root string) (map[string]project.Job, error) {
	s, err := settings(root)
	if err != nil {
		return nil, err
	}
	// project refuses a job of the repo's named as charter's is (project.CharterJobs, these).
	out := maps.Clone(charterJobs)
	maps.Copy(out, s.CI.Jobs)
	return out, nil
}

// workflowOf is a job's workflow: charter-<job>.yml for charter's, self-<job>.yml for the repo's own.
func workflowOf(name string) string {
	if _, ok := charterJobs[name]; ok {
		return "charter-" + name + ".yml"
	}
	return "self-" + name + ".yml"
}

// notSources are the folders projectsOf does not look in: what is installed, built or charter's.
var notSources = []string{"node_modules", "build", "dist", "target", "out", "sdk"}

// projectsOf are the projects a job runs in: those it names, or every folder of the repo with a charter.toml naming a
// Worker, by path.
func projectsOf(root string, j project.Job) []string {
	if len(j.Projects) > 0 {
		return slices.Clone(j.Projects)
	}
	var out []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && p != root && (strings.HasPrefix(d.Name(), ".") || slices.Contains(notSources, d.Name())) {
			return filepath.SkipDir
		}
		if d.IsDir() || d.Name() != project.File || filepath.Dir(p) == root {
			return nil
		}
		if pr, err := project.LoadRepo(filepath.Dir(p)); err == nil && pr.IsWorker() {
			rel, _ := filepath.Rel(root, filepath.Dir(p))
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(out)
	return out
}

var runners = map[string]string{"linux": "ubuntu-24.04", "macos": "macos-15", "windows": "windows-2025"}

// runner is the job runner a repo's workflows call: its own copy, which charter init writes (init -check fails while
// it differs), so it moves with that charter.
const runner = "./.github/workflows/charter-ci.yml"

// workflow is a job's workflow: when it runs, and a call of the shared runner per project and system. Every workflow
// can be started by hand, so from here: charter ci <job> [<project>] -remote.
func workflow(root, name string, j project.Job) string {
	systems := j.OS
	if len(systems) == 0 {
		systems = []string{"linux"}
	}
	var os []string
	for _, s := range systems {
		os = append(os, runners[s])
	}
	var projects []string
	if j.Each == "project" {
		projects = projectsOf(root, j)
	}
	byHand := len(j.On) == 1 && j.On[0] == "dispatch"
	one := name
	if len(projects) > 0 {
		one += " [<project>]"
	}
	mark := "Written by `charter init` from charter.toml: don't edit, change charter.toml's [ci.jobs], then charter init."
	if _, ok := charterJobs[name]; ok {
		mark = "Written by `charter init` (github.com/joeblew999/charter): don't edit, it is charter's job, the same in every repo."
	}
	lines := []string{"# " + mark, fmt.Sprintf("# %s. Here: charter ci %s; on GitHub, from here: charter ci %s -remote.", j.About, one, one), "name: " + strings.TrimSuffix(workflowOf(name), ".yml"), "on:"}
	var push []string
	if slices.Contains(j.On, "push") {
		push = append(push, "    branches: [main]")
	}
	if slices.Contains(j.On, "tag") {
		push = append(push, `    tags: ["v*"]`)
	}
	if len(push) > 0 {
		lines = append(append(lines, "  push:"), push...)
	}
	if slices.Contains(j.On, "pull_request") {
		lines = append(lines, "  pull_request:")
	}
	lines = append(lines, "  workflow_dispatch:")
	switch {
	case len(projects) > 0 && byHand:
		lines = append(lines, "    inputs:", "      project:", "        description: The project", "        type: choice", "        required: true", "        options: ["+strings.Join(projects, ", ")+"]")
	case len(projects) > 0:
		lines = append(lines, "    inputs:", "      project:", "        description: One project (none, every project)", "        type: string", `        default: ""`)
	}
	lines = append(lines, "jobs:", "  "+name+":")
	list, _ := json.Marshal(projects)
	var matrix []string
	if len(projects) > 0 {
		matrix = append(matrix, fmt.Sprintf(`        project: ${{ fromJSON(inputs.project && format('["{0}"]', inputs.project) || '%s') }}`, list))
	}
	if len(os) > 1 {
		matrix = append(matrix, "        os: ["+strings.Join(os, ", ")+"]")
	}
	if len(matrix) > 0 {
		lines = append(append(lines, "    strategy:", "      fail-fast: false", "      matrix:"), matrix...)
	}
	if len(j.Permissions) > 0 {
		lines = append(lines, "    permissions:")
		for _, scope := range []string{"contents", "issues", "pull-requests", "pages", "actions", "id-token", "packages", "deployments", "checks", "statuses"} {
			if access, ok := j.Permissions[scope]; ok {
				lines = append(lines, "      "+scope+": "+access)
			}
		}
	}
	lines = append(lines, "    uses: "+runner, "    with:", "      job: "+name)
	if len(projects) > 0 {
		lines = append(lines, "      project: ${{ matrix.project }}")
	}
	on := os[0]
	if len(os) > 1 {
		on = "${{ matrix.os }}"
	}
	timeout := j.Timeout
	if timeout == 0 {
		timeout = 60
	}
	lines = append(lines, "      runs-on: "+on, "      timeout-minutes: "+strconv.Itoa(timeout))
	if len(j.Secrets) > 0 {
		lines = append(lines, "    secrets: inherit")
	}
	return strings.Join(lines, "\n") + "\n"
}

// onEveryChange are the jobs GitHub runs on a push or a pull request (not one run by hand), by name.
func onEveryChange(jobs map[string]project.Job) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(jobs)) {
		if slices.Contains(jobs[name].On, "push") || slices.Contains(jobs[name].On, "pull_request") {
			out = append(out, name)
		}
	}
	return out
}

// passedSecrets are the repo's secrets the shared runner passes on GitHub (CHARTER_SECRETS), taken out of the
// environment, so a job's commands get only those it names.
func passedSecrets() map[string]string {
	passed := map[string]string{}
	if s := os.Getenv("CHARTER_SECRETS"); s != "" {
		json.Unmarshal([]byte(s), &passed)
	}
	os.Unsetenv("CHARTER_SECRETS")
	return passed
}

// secretsFor are the secrets a job names, from the environment, or on GitHub from those passed: a job gets those it
// names and no others.
func secretsFor(j project.Job, passed map[string]string) map[string]string {
	out := map[string]string{}
	for _, name := range j.Secrets {
		if v := os.Getenv(name); v != "" {
			out[name] = v
		} else if v := passed[name]; v != "" {
			out[name] = v
		}
	}
	return out
}

// self is this charter, which a job's command named charter runs as.
func self() string {
	bin, err := os.Executable()
	if err != nil {
		return "charter"
	}
	return bin
}

// exec runs a job's command in dir, charter as this charter, with the terminal's output.
func execCommand(line, dir string, env map[string]string) error {
	words := strings.Fields(line)
	if len(words) == 0 {
		return errors.New("an empty command in a job")
	}
	if words[0] == "charter" {
		words[0] = self()
	}
	return run.Run(dir, env, words[0], words[1:]...)
}

// RunJobs runs the jobs named, one after another, in root or in each project, and says how long each took.
func RunJobs(root string, names []string, project string) error {
	jobs, err := Jobs(root)
	if err != nil {
		return err
	}
	passed := passedSecrets()
	var times []string
	for _, name := range names {
		j, ok := jobs[name]
		if !ok {
			return fmt.Errorf("charter.toml has no job %s: %s", name, strings.Join(slices.Sorted(maps.Keys(jobs)), ", "))
		}
		start := time.Now()
		dirs := []string{"."}
		if j.Each == "project" {
			dirs = projectsOf(root, j)
			if project != "" {
				dirs = []string{project}
			}
		}
		env := secretsFor(j, passed)
		for _, dir := range dirs {
			fmt.Printf("\n==== ci %s%s\n", name, map[bool]string{true: "", false: " " + dir}[dir == "."])
			for _, line := range j.Run {
				if err := execCommand(line, filepath.Join(root, dir), env); err != nil {
					return err
				}
			}
		}
		times = append(times, name+" "+run.Seconds(time.Since(start)))
	}
	fmt.Printf("\nci passed: %s\n", strings.Join(times, ", "))
	return nil
}

// CheckJobs runs the jobs charter.toml's [ci] check lists (by default every job run on a change), as charter check
// does at a repo's top.
func CheckJobs(root string) error {
	s, err := settings(root)
	if err != nil {
		return err
	}
	names := s.CI.Check
	if len(names) == 0 {
		jobs, err := Jobs(root)
		if err != nil {
			return err
		}
		names = onEveryChange(jobs)
	}
	return RunJobs(root, names, "")
}

// HasJobs is whether the repo at root has jobs of its own.
func HasJobs(root string) bool {
	s, err := settings(root)
	return err == nil && len(s.CI.Jobs) > 0
}

// CI runs charter ci [<job> [<project>]] [-remote]: a job here, as GitHub runs it, or on GitHub; with no job, every
// job GitHub runs on a change.
func CI(args []string, remote bool) error {
	r, err := Root()
	if err != nil {
		return err
	}
	if remote {
		return ciRemote(r, args)
	}
	if len(args) == 0 {
		jobs, err := Jobs(r)
		if err != nil {
			return err
		}
		return RunJobs(r, onEveryChange(jobs), "")
	}
	project := ""
	if len(args) > 1 {
		project = args[1]
	}
	return RunJobs(r, args[:1], project)
}

// ciRemote starts a job's workflow on GitHub, on this branch (pushed), and says where the run is. It does not wait
// for it: gh run watch follows it.
func ciRemote(root string, args []string) error {
	jobs, err := Jobs(root)
	if err != nil {
		return err
	}
	if len(args) == 0 || jobs[args[0]].Run == nil {
		return fmt.Errorf("ci -remote needs a job of charter.toml: %s", strings.Join(slices.Sorted(maps.Keys(jobs)), ", "))
	}
	name := args[0]
	branch, err := run.Output(root, nil, "git", "branch", "--show-current")
	if err != nil {
		return err
	}
	branch = strings.TrimSpace(branch)
	started := time.Now().Add(-5 * time.Second).UTC().Format(time.RFC3339)
	start := []string{"workflow", "run", workflowOf(name), "--ref", branch}
	if len(args) > 1 {
		start = append(start, "-f", "project="+args[1])
	}
	if err := run.Run(root, nil, "gh", start...); err != nil {
		return err
	}
	for range 15 {
		out, err := gh(root, "run", "list", "--workflow", workflowOf(name), "--branch", branch, "--event", "workflow_dispatch", "--limit", "1", "--json", "url,createdAt,databaseId")
		var runs []struct {
			URL       string `json:"url"`
			CreatedAt string `json:"createdAt"`
			ID        int    `json:"databaseId"`
		}
		if err == nil && json.Unmarshal([]byte(out), &runs) == nil && len(runs) > 0 && runs[0].CreatedAt >= started {
			fmt.Printf("started: %s\nnot waiting for it (gh run watch %d follows it)\n", runs[0].URL, runs[0].ID)
			return nil
		}
		time.Sleep(time.Second)
	}
	fmt.Printf("started %s on %s; gh run list --workflow %s shows it\n", workflowOf(name), branch, workflowOf(name))
	return nil
}
