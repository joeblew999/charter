package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// charter flow: the whole flow, on throwaway things, every step shown as it runs.
// Run by: mise run flow (mise.toml)
func init() {
	commands["flow"] = command{"[-no-project] [-no-deploy]",
		"run the whole flow on throwaway things, every task of tasks/repo and tasks/project in the order the files have them: the repo's on a local repo (with -github, on <you>/charter-flow-test on GitHub, deleted and made again), then a project's on one charter new makes, built, with its SDKs, deployed to the Worker charter-flow-api and deleted after; a step it cannot run is named, with why; a summary at the end", flowCommand}
	anywhere["flow"] = true
}

func init() {
	commands["flow-clean"] = command{"<worker>", "REMOTE: delete the throwaway Worker charter flow deployed (charter-flow-*) and its database", func(args []string) error {
		if len(args) != 1 {
			return errors.New("flow-clean needs the Worker: charter-flow-api")
		}
		return deleteScratch(args[0])
	}}
	anywhere["flow-clean"] = true
}

// flowStep is a step that ran, and how it went.
type flowStep struct {
	label, result string
}

func flowCommand(args []string) error {
	var noProject, noDeploy, github, noLocal, remote bool
	flags("flow", args, func(f *flag.FlagSet) {
		f.BoolVar(&noProject, "no-project", false, "run the repo's flow only")
		f.BoolVar(&noDeploy, "no-deploy", false, "leave out the project's deploy and the Cloudflare steps")
		f.BoolVar(&noLocal, "no-local", false, "leave out every mise file in this checkout (run in place)")
		f.BoolVar(&remote, "remote", false, "run the REMOTE tasks of the mise files in this checkout too: the examples' own Workers, GitHub, Cloudflare")
		f.BoolVar(&github, "github", false, "run the steps that work on GitHub too, on a throwaway repo there (<you>/charter-flow-test)")
	})
	checkout, err := output(started, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return errors.New("flow runs in a checkout of charter")
	}
	owner := "flow"
	if github {
		login, err := gh(checkout, "api", "user", "--jq", ".login")
		if err != nil {
			return err
		}
		owner = login
	}
	work, err := os.MkdirTemp("", "charter-flow-")
	if err != nil {
		return err
	}
	fmt.Println("flow: working in", work)
	bin := filepath.Join(work, "bin")
	if err := sh(checkout, "go", "build", "-o", filepath.Join(bin, exe("charter")), "./cmd/charter"); err != nil {
		return err
	}
	var steps []flowStep
	run := func(dir, label, program string, args ...string) bool {
		fmt.Printf("\n==== %s   (in %s)\n", label, dir)
		cmd := exec.Command(program, args...)
		cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "MISE_TRUSTED_CONFIG_PATHS="+dir)
		err := cmd.Run()
		result := "ok"
		if err != nil {
			result = "FAILED: " + err.Error()
		}
		steps = append(steps, flowStep{label, result})
		return err == nil
	}
	skip := func(label, why string) {
		fmt.Printf("\n==== %s: not run: %s\n", label, why)
		steps = append(steps, flowStep{label, "not run: " + why})
	}
	tasksOf := func(file string) []string {
		content, _ := os.ReadFile(filepath.Join(checkout, file))
		names, _ := tomlTasks(content)
		return names
	}

	// 1. The repo's flow (tasks/repo), on a repo on GitHub with files of its own.
	repo, dir := owner+"/charter-flow-test", filepath.Join(work, "charter-flow-test")
	if github {
		gh(checkout, "repo", "delete", repo, "--yes")
	}
	own := map[string]string{
		"README.md":                      "# charter-flow-test\n\nThrowaway: charter flow makes it again on every run.\n",
		"charter.toml":                   "description = \"Throwaway: charter flow makes it again on every run\"\nworkflow = false\n",
		"docs/README.md":                 "---\ntitle: Home\nnav_order: 1\npermalink: /\n---\n\n# Our docs\n\n[Our guide](guide.md). [Rules](rules.md).\n",
		"docs/guide.md":                  "---\ntitle: Guide\nnav_order: 2\n---\n\n# Our guide\n",
		".github/ISSUE_TEMPLATE/bug.yml": "name: Our bug form\ndescription: ours\nbody:\n  - type: textarea\n    attributes:\n      label: What\n",
		"CLAUDE.md":                      "# Our notes\n\nAlways run the tests.\n",
	}
	for file, content := range own {
		if err := write(filepath.Join(dir, filepath.FromSlash(file)), content); err != nil {
			return err
		}
	}
	pushed := false
	commit := func(message string) {
		run(dir, "git add", "git", "add", "-A")
		run(dir, "git commit: "+message, "git", "-c", "user.name=charter-flow", "-c", "user.email=flow@charter.invalid", "commit", "-q", "-m", message)
		if pushed {
			run(dir, "git push", "git", "push", "-q")
		}
	}
	run(dir, "git init", "git", "init", "-q", "-b", "main")
	commit("a repo with files of its own")
	if github {
		pushed = run(dir, "gh repo create "+repo, "gh", "repo", "create", repo, "--public", "--source", ".", "--push")
	}
	run(dir, "charter adopt (the first time: the tool from this checkout)", "charter", "adopt", "-from", checkout)
	for _, task := range tasksOf("tasks/repo/tasks.toml") {
		switch task {
		case "repo", "repo:check", "repo:ci", "issues", "release":
			if !github {
				skip("mise run "+task, "it works on the repo on GitHub (charter flow -github runs it, on a throwaway repo there)")
				continue
			}
			if task == "release" {
				commit("charter adopt, the steps of the flow")
				run(dir, "mise run release -- v0.0.1", "mise", "run", "release", "--", "v0.0.1")
				continue
			}
			run(dir, "mise run "+task, "mise", "run", task)
		case "docs:review", "docs:pages":
			skip("mise run "+task, "a step for now and then (Claude reviews the docs; Pages is on already, from mise run repo)")
		case "issue":
			run(dir, "mise run issue -- plan", "mise", "run", "issue", "--", "plan")
		default:
			run(dir, "mise run "+task, "mise", "run", task)
		}
	}

	// 2. A project's flow (tasks/project), on one charter new makes.
	if noProject {
		skip("the project's flow", "-no-project")
	} else {
		name := "charter-flow-api"
		project := filepath.Join(work, name)
		if run(work, "charter new "+name, "charter", "new", "-name", name, "-empty", "-module", "github.com/"+owner+"/"+name, "-into", project, "-from", checkout) {
			run(project, "git init", "git", "init", "-q", "-b", "main")
			deployed := false
			for _, task := range tasksOf("tasks/project/tasks.toml") {
				switch task {
				case "run", "dev", "migrate:local", "sdk:docs":
					skip("mise run "+task, "it starts a server and keeps running (the project's own live tests run it)")
				case "sdk:cli:build", "sdk:cli:smoke", "sdk:dist:cli":
					skip("mise run "+task, "the project has no CLI (charter new -empty)")
				case "sdk:gen", "sdk:check":
					run(project, "mise run "+task+" -- go", "mise", "run", task, "--", "go")
				case "cloudflare:secrets", "access:setup", "access:token", "access:delete":
					skip("mise run "+task, "it changes your GitHub secrets or your account's Cloudflare Access, which a throwaway project must not")
				case "spec:diff", "release:publish", "release:tags":
					skip("mise run "+task, "the project has no release (the repo's flow cuts one)")
				case "perf", "perf:try", "perf:clean":
					skip("mise run "+task, "a step for now and then (it deploys its own scratch Worker)")
				case "cloudflare:token", "migrate", "tail":
					switch {
					case noDeploy:
						skip("mise run "+task, "-no-deploy")
					case !deployed:
						deployed = true
						run(project, "mise run deploy (the project's own task: the Worker "+name+")", "mise", "run", "deploy")
						fallthrough
					default:
						if task == "tail" {
							run(project, "mise run tail -- -for 10s", "mise", "run", "tail", "--", "-for", "10s")
						} else {
							run(project, "mise run "+task, "mise", "run", task)
						}
					}
				default:
					run(project, "mise run "+task, "mise", "run", task)
				}
			}
			if deployed {
				fmt.Printf("\n==== delete the Worker %s and its database\n", name)
				cmd := exec.Command(os.Args[0], "flow-clean", name)
				cmd.Dir, cmd.Stdout, cmd.Stderr = project, os.Stdout, os.Stderr
				result := "ok"
				if err := cmd.Run(); err != nil {
					result = "FAILED: " + err.Error()
				}
				steps = append(steps, flowStep{"delete the Worker " + name, result})
			}
		}
	}

	// 3. Every mise file in this checkout, in place: each project's tasks (tasks/project, then its own
	// mise.toml), then charter's own (tasks/repo, then its mise.toml), every task in the order the
	// files have them. A REMOTE task (its description says so) runs only with -remote.
	if noLocal {
		skip("every mise file in this checkout", "-no-local")
	} else {
		projects, _ := projectsBelow(checkout)
		places := append(projects, checkout)
		for _, place := range places {
			own := tasksOf(rel(checkout, filepath.Join(place, "mise.toml")))
			var order []string
			if place == checkout {
				order = append(tasksOf("tasks/repo/tasks.toml"), own...)
			} else {
				order = append(tasksOf("tasks/project/tasks.toml"), own...)
			}
			described := taskDescriptions(place)
			where := rel(checkout, place)
			if where == "." {
				where = "charter"
			}
			seen := map[string]bool{}
			for _, task := range order {
				if seen[task] {
					continue
				}
				seen[task] = true
				label := where + ": mise run " + task
				switch {
				case strings.HasPrefix(described[task], "REMOTE") && !remote:
					skip(label, "REMOTE (charter flow -remote runs it)")
				case task == "run" || task == "dev" || task == "sdk:docs" || task == "mcp-test" || task == "migrate:local":
					skip(label, "a server: test:native and test:workerd start it, test it and stop it")
				case task == "check" || task == "projects:check" || task == "flow":
					skip(label, "it runs the steps above again")
				case task == "docs:review":
					skip(label, "it has Claude edit the docs")
				case task == "issue":
					run(place, label+" -- plan", "mise", "run", task, "--", "plan")
				case task == "sdk:gen" || task == "sdk:check":
					run(place, label+" -- go", "mise", "run", task, "--", "go")
				case strings.HasPrefix(task, "sdk:cli:") || task == "sdk:dist:cli":
					if shipsCLI(place) {
						run(place, label, "mise", "run", task)
					} else {
						skip(label, "the project has no CLI")
					}
				case task == "tail":
					run(place, label+" -- -for 10s", "mise", "run", task, "--", "-for", "10s")
				default:
					run(place, label, "mise", "run", task)
				}
			}
		}
	}

	failed := 0
	fmt.Println("\n==== the flow, step by step")
	for _, s := range steps {
		fmt.Printf("%-66s %s\n", s.label, s.result)
		if strings.HasPrefix(s.result, "FAILED") {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d steps failed (above); what they left is in %s", failed, work)
	}
	fmt.Println("every step that ran passed; what it made is in", work)
	return nil
}

// rel is path below root, with forward slashes.
func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

// taskDescriptions are the descriptions of the tasks mise offers in dir, by name.
func taskDescriptions(dir string) map[string]string {
	descriptions := map[string]string{}
	cmd := exec.Command("mise", "tasks", "ls", "--json")
	cmd.Dir, cmd.Env = dir, append(os.Environ(), "MISE_TRUSTED_CONFIG_PATHS="+dir)
	out, err := cmd.Output()
	if err != nil {
		return descriptions
	}
	var tasks []struct{ Name, Description string }
	if json.Unmarshal(out, &tasks) == nil {
		for _, t := range tasks {
			descriptions[t.Name] = t.Description
		}
	}
	return descriptions
}
