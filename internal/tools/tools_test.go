package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// The program is taken out of a release's archive by its name, wherever it sits in it: a .tar.gz, and Windows' .zip.
func TestMember(t *testing.T) {
	var tgz bytes.Buffer
	gz := gzip.NewWriter(&tgz)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "fnox-1/README.md", Mode: 0o644, Size: 2, Typeflag: tar.TypeReg})
	tw.Write([]byte("hi"))
	tw.WriteHeader(&tar.Header{Name: "fnox-1/fnox", Mode: 0o755, Size: 6, Typeflag: tar.TypeReg})
	tw.Write([]byte("binary"))
	tw.Close()
	gz.Close()
	if got, err := member(tgz.Bytes(), "fnox", false); err != nil || string(got) != "binary" {
		t.Fatalf("tar.gz: %q %v", got, err)
	}
	var z bytes.Buffer
	zw := zip.NewWriter(&z)
	w, _ := zw.Create("fnox.exe")
	w.Write([]byte("windows"))
	zw.Close()
	if got, err := member(z.Bytes(), "fnox.exe", true); err != nil || string(got) != "windows" {
		t.Fatalf("zip: %q %v", got, err)
	}
	if _, err := member(z.Bytes(), "other.exe", true); err == nil {
		t.Fatal("a zip without the program gave one")
	}
}

// Every system charter runs on has its fnox, with a whole SHA-256.
func TestEverySystemHasItsFnox(t *testing.T) {
	for _, system := range []string{"darwin/arm64", "darwin/amd64", "linux/arm64", "linux/amd64", "windows/arm64", "windows/amd64"} {
		if f, ok := fnoxFiles[system]; !ok || len(f[1]) != 64 {
			t.Errorf("%s: %v", system, f)
		}
	}
	if _, ok := fnoxFiles[runtime.GOOS+"/"+runtime.GOARCH]; !ok {
		t.Errorf("no fnox for this machine, %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// On each system charter runs on (the workflow "systems" sets CHARTER_TOOLS_TEST there, weekly): charter brings its
// fnox from the release, checked against its SHA-256, and the program runs.
func TestFnoxIsBrought(t *testing.T) {
	if os.Getenv("CHARTER_TOOLS_TEST") == "" {
		t.Skip("CHARTER_TOOLS_TEST is not set: this downloads fnox")
	}
	bin, err := Fnox()
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "fnox") {
		t.Fatalf("%s --version: %v\n%s", bin, err, out)
	}
	t.Logf("%s/%s: %s", runtime.GOOS, runtime.GOARCH, strings.TrimSpace(string(out)))
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
