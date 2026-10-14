package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// What charter keeps in a repo is one tree, cmd/charter/kit, laid out as the repo has it: its own
// pages in docs/charter/, the docs site's files that GitHub Pages reads at the top of docs/, and the
// issue forms and the workflow in .github/, each named for charter. The same in every repo, charter's
// own included. Charter writes a file only where it may (keepCharters): where there is none, or the
// one there carries its mark. What a repo wrote itself is never written over, whatever its path.

//go:embed all:kit
var kit embed.FS

// charterFolder is charter's own folder in the docs: every page charter writes is in it.
const charterFolder = "charter"

// kitFiles are the kit's files below dir (docs, .github), by their path below it.
func kitFiles(dir string) map[string][]byte {
	files := map[string][]byte{}
	fs.WalkDir(kit, "kit/"+dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		files[strings.TrimPrefix(path, "kit/"+dir+"/")], err = kit.ReadFile(path)
		return err
	})
	return files
}

// kitFile is one file of the kit, by its path in a repo.
func kitFile(path string) string {
	content, err := kit.ReadFile("kit/" + path)
	if err != nil {
		panic("the kit has no " + path) // a test holds every name used here to the tree
	}
	return string(content)
}

// docsFiles are the files charter writes into the repo's docs folder, by their path in the repo: its
// pages in the folder of its own (the page on using charter is written from the tool itself), and
// the site's files at the folder's top. The site's config holds the repo's name and description on
// GitHub: without the repo (nil) it is left out. The tasks page is not here: mise writes it (tasksFile).
func docsFiles(folder string, repo *githubRepo) map[string][]byte {
	files := map[string][]byte{}
	for path, content := range kitFiles("docs") {
		if path == "_config.yml" {
			if repo == nil {
				continue
			}
			description, _ := json.Marshal(repo.Description) // a YAML string, safely quoted
			content = []byte(strings.NewReplacer("__NAME__", repo.Name, "__DESCRIPTION__", string(description),
				"__REPO__", repo.NameWithOwner, "__BRANCH__", repo.DefaultBranchRef.Name,
				"gh_edit_source: docs", "gh_edit_source: "+folder).Replace(string(content)))
		}
		files[filepath.Join(folder, filepath.FromSlash(path))] = content
	}
	files[filepath.Join(folder, charterFolder, "using.md")] = []byte(docsUsing())
	return files
}

// tasksFile is the tasks page in charter's folder, by its path in the repo: the repo's tasks as mise
// lists them (charter docs-tasks), under the title and the line that say whose it is. It fails
// where mise cannot list them yet (the tool or an include not installed).
func tasksFile(root, folder string) (string, []byte, error) {
	out, err := docsTasksPage(root, nil)
	if err != nil {
		return "", nil, err
	}
	title, rest, _ := strings.Cut(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	note := "Written by `charter docs` from the repo's mise tasks (`charter docs-tasks`): don't edit, change the tasks, then `mise run docs:setup`."
	page := "---\ntitle: Tasks\nnav_order: 4\nparent: How this repo is kept\n---\n\n" + title + "\n\n" + note + "\n" + strings.TrimRight(rest, "\n") + "\n"
	return filepath.Join(folder, charterFolder, "tasks.md"), []byte(page), nil
}

// formFiles are the issue forms, by their path below .github.
func formFiles() map[string][]byte {
	files := map[string][]byte{}
	for path, content := range kitFiles(".github") {
		if strings.HasPrefix(path, "ISSUE_TEMPLATE/") {
			files[path] = content
		}
	}
	return files
}

// keepCharters writes those of files (by their path below dir) that differ from what is there, where
// charter may: in its own places (ownPlace) always; elsewhere where there is no file, or the one there
// carries charter's mark (Written by ... don't edit). One without it is the repo's own: kept, and
// named in kept. With check it writes nothing and only names them.
func keepCharters(dir string, files map[string][]byte, check bool) (stale, kept []string, err error) {
	for name, content := range files {
		path := filepath.Join(dir, name)
		have, err := os.ReadFile(path)
		switch {
		case err == nil && bytes.Equal(have, content):
			continue
		case err == nil && !ownPlace(name):
			if charters, _, ok := markOf(have); !ok || !charters {
				kept = append(kept, filepath.ToSlash(name))
				continue
			}
		}
		stale = append(stale, filepath.ToSlash(name))
		if !check {
			if err := write(path, string(content)); err != nil {
				return nil, nil, err
			}
		}
	}
	slices.Sort(stale)
	slices.Sort(kept)
	return stale, kept, nil
}

// ownPlace reports whether a path charter writes is in one of its own places, which are charter's
// whatever is in them: its folder in the docs, its issue forms (charter-*.yml), its workflow
// (charter.yml). The other files it writes have names the host fixes (docs/_config.yml, AGENTS.md).
func ownPlace(path string) bool {
	path = filepath.ToSlash(path)
	base := filepath.Base(path)
	return strings.HasPrefix(path, charterFolder+"/") || strings.Contains(path, "/"+charterFolder+"/") ||
		strings.HasPrefix(base, "charter-") || base == repoWorkflow
}

// sweep removes the files in charter's own place dir (below root) that it does not write (files, by
// their path below root), so the place is exactly what this release writes: a new release needs no
// steps of its own. Only names that match are charter's (nil: every file in dir). With check it only
// names them.
func sweep(root, dir string, match func(name string) bool, files map[string][]byte, check bool) ([]string, error) {
	var gone []string
	err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil || entry.IsDir() || (match != nil && !match(entry.Name())) {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for name := range files {
			if filepath.ToSlash(name) == filepath.ToSlash(rel) {
				return nil
			}
		}
		gone = append(gone, "removed "+filepath.ToSlash(rel))
		if check {
			return nil
		}
		return os.Remove(path)
	})
	return gone, err
}

// isCharter reports whether the repo at root is charter's own: its tasks run the tool from the
// checkout, so it pins none and includes tasks/repo by its folder.
func isCharter(root string) bool {
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	return err == nil && regexp.MustCompile(`(?m)^module `+regexp.QuoteMeta(repoModule)+`\s*$`).Match(mod)
}
