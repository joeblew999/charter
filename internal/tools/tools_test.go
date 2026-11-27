package tools

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// On each system charter runs on (the workflow "systems" sets CHARTER_TOOLS_TEST there, weekly, after mise has
// installed the repository's tools): fnox is mise's, and the program runs.
func TestFnoxIsMises(t *testing.T) {
	if os.Getenv("CHARTER_TOOLS_TEST") == "" {
		t.Skip("CHARTER_TOOLS_TEST is not set: this asks mise")
	}
	bin, err := Fnox()
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "fnox") {
		t.Fatalf("%s --version: %v\n%s", bin, err, out)
	}
	t.Logf("%s/%s: %s (%s)", runtime.GOOS, runtime.GOARCH, strings.TrimSpace(string(out)), bin)
}

// Where a container engine already answers (a CI runner has Docker), charter uses it and installs nothing.
func TestAnEngineThatAnswersIsUsed(t *testing.T) {
	if os.Getenv("CHARTER_TOOLS_TEST") == "" || exec.Command("docker", "info").Run() != nil {
		t.Skip("CHARTER_TOOLS_TEST is not set, or no container engine answers here")
	}
	if _, err := Containers(); err != nil {
		t.Fatal(err)
	}
}
