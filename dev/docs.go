package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	commands["docs"] = command{"[-into <repo dir>] [-check]",
		"write the docs site's config into a repo's docs/ folder (GitHub Pages renders it; any repo, same look)", docs}
}

//go:embed docs/_config.yml
var docsConfig string

//go:embed docs/custom.scss
var docsStyle string

// docs writes the two files that make a repo's docs/ folder a site on GitHub Pages. They are the
// same for every repo except for its name, description and URLs, which GitHub is asked for. With
// -check nothing is written, and it fails if a file differs.
func docs(args []string) error {
	into, check := ".", false
	flags("docs", args, func(f *flag.FlagSet) {
		f.StringVar(&into, "into", into, "the repo to write into (its docs/ folder)")
		f.BoolVar(&check, "check", false, "write nothing; fail if a file differs")
	})
	out, err := output(into, "gh", "repo", "view", "--json", "nameWithOwner,name,description,defaultBranchRef")
	if err != nil {
		return errors.New("docs needs a GitHub repo here (gh repo view failed)")
	}
	var repo struct {
		NameWithOwner, Name, Description string
		DefaultBranchRef                 struct{ Name string }
	}
	if err := json.Unmarshal([]byte(out), &repo); err != nil {
		return err
	}
	description, _ := json.Marshal(repo.Description) // a YAML string, safely quoted
	config := strings.NewReplacer("__NAME__", repo.Name, "__DESCRIPTION__", string(description),
		"__REPO__", repo.NameWithOwner, "__BRANCH__", repo.DefaultBranchRef.Name).Replace(docsConfig)
	stale := 0
	for path, content := range map[string]string{"docs/_config.yml": config, "docs/_sass/custom/custom.scss": docsStyle} {
		path = filepath.Join(into, path)
		if current, _ := os.ReadFile(path); bytes.Equal(current, []byte(content)) {
			continue
		}
		stale++
		if check {
			fmt.Fprintf(os.Stderr, "%s differs from what `dev docs` writes: mise run docs:setup\n", path)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Println("wrote", path)
	}
	if check && stale > 0 {
		return errors.New("the docs site's config is stale")
	}
	owner, name, _ := strings.Cut(repo.NameWithOwner, "/")
	fmt.Printf("docs site: https://%s.github.io/%s/ (first time: mise run docs:pages)\n", owner, name)
	return nil
}
