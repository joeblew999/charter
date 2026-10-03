package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A Docker that says nothing (asleep with the Mac) is named as such in seconds, not waited for.
func TestDockerThatDoesNotAnswer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in docker is a shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	before := dockerWait
	dockerWait = 300 * time.Millisecond
	defer func() { dockerWait = before }()
	start := time.Now()
	err := docker()
	if err == nil || !strings.Contains(err.Error(), "doesn't answer") {
		t.Fatalf("docker() = %v, want it named as not answering", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("took %s", took)
	}
}
