package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The patched TinyGo root is made of the installed one's files: copies, or hard links where the
// system has no symbolic links for this user (Windows). A patch must then change the copy only.
func TestTreeCopiesOrLinksAFolder(t *testing.T) {
	from := t.TempDir()
	for file, content := range map[string]string{"a.go": "package a\n", "deep/er/b.go": "package b\n"} {
		if err := write(filepath.Join(from, file), content); err != nil {
			t.Fatal(err)
		}
	}
	for name, link := range map[string]bool{"copied": false, "linked": true} {
		to := filepath.Join(t.TempDir(), name)
		if err := tree(from, to, link); err != nil {
			t.Fatal(err)
		}
		for file, want := range map[string]string{"a.go": "package a\n", "deep/er/b.go": "package b\n"} {
			if got, err := os.ReadFile(filepath.Join(to, file)); err != nil || string(got) != want {
				t.Errorf("%s: %s is %q (%v), want %q", name, file, got, err, want)
			}
		}
		// As a patch is written: the file is removed, then written, so a link is not written through.
		patched := filepath.Join(to, "a.go")
		if err := os.Remove(patched); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(patched, []byte("package patched\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(filepath.Join(from, "a.go")); string(got) != "package a\n" {
			t.Errorf("%s: patching the copy changed the original: %q", name, got)
		}
	}
}
