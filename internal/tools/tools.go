// Package tools is the two programs charter picks for every developer, beside Go and Node, and brings itself when a
// machine lacks them, so that nobody sets either up or has to remember to:
//
//   - fnox (https://fnox.jdx.dev) keeps secrets: the Cloudflare token a deploy needs comes from the keychain or a
//     password manager, never from a file or the shell's history. charter runs wrangler under fnox exec.
//   - Kiri (https://runkiri.com) runs containers, which Fern's SDK generators are: charter pack uses its docker.
//     Where another Docker engine already answers (CI's own, a server's), that one is used and Kiri is left alone.
//
// charter install brings both; a command that needs one brings it then, if install was never run.
package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// fnoxVersion is the fnox charter brings, and fnoxFiles its release file for each system with the file's SHA-256:
// what is downloaded is that file and no other. A new fnox is a new version here with its sums.
const fnoxVersion = "1.50.0"

var fnoxFiles = map[string][2]string{
	"darwin/arm64":  {"fnox-aarch64-apple-darwin.tar.gz", "cf8301a5355c3249284b4102c871b0c4b5e5924cd7b61397f7539f4328041eff"},
	"darwin/amd64":  {"fnox-x86_64-apple-darwin.tar.gz", "76bd810eaecdce2e02fcbe51deda070e72dcf85c216d746512c469f4012e7f72"},
	"linux/arm64":   {"fnox-aarch64-unknown-linux-musl.tar.gz", "6dd97b15ca535e52813ad01f4e6e14e0b630fcd18cfaf439a8b9dd76ffd74e85"},
	"linux/amd64":   {"fnox-x86_64-unknown-linux-musl.tar.gz", "3ad4c13244cd9fb6ae45931cc22f876a3ee8608a0fcc7e429c2f890ff6d7da18"},
	"windows/arm64": {"fnox-aarch64-pc-windows-msvc.zip", "d7cb1ea29de4935995db0672e123a236f39ae2588f3f827ebf62d5728b11b2d2"},
	"windows/amd64": {"fnox-x86_64-pc-windows-msvc.zip", "14ddd4b9d97274e5ec725ff98fbf3145291c94d21ef40c6ffa224900ad6d15e6"},
}

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// Fnox is the path of fnox: charter's own copy (in the user's cache, at the version above), downloaded the first
// time. It is charter's and not whichever fnox the PATH has, so that every developer and every machine runs the same
// one, and a version manager's shim with nothing behind it changes nothing.
func Fnox() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "charter", "tools", "fnox-"+fnoxVersion)
	bin := filepath.Join(dir, exe("fnox"))
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	file, ok := fnoxFiles[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("fnox has no release for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	url := "https://github.com/jdx/fnox/releases/download/v" + fnoxVersion + "/" + file[0]
	fmt.Printf("fnox %s is not on this machine yet: getting %s\n", fnoxVersion, url)
	res, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: HTTP %d", url, res.StatusCode)
	}
	archive, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if sum := sha256.Sum256(archive); hex.EncodeToString(sum[:]) != file[1] {
		return "", fmt.Errorf("%s is not the file charter expects (SHA-256 %s, want %s): nothing was installed", url, hex.EncodeToString(sum[:]), file[1])
	}
	program, err := member(archive, exe("fnox"), strings.HasSuffix(file[0], ".zip"))
	if err != nil {
		return "", fmt.Errorf("%s: %w", file[0], err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Written whole, then named: a download that was cut off leaves no fnox behind.
	if err := os.WriteFile(bin+".part", program, 0o755); err != nil {
		return "", err
	}
	return bin, os.Rename(bin+".part", bin)
}

// member is the file called name in a .tar.gz or .zip archive.
func member(archive []byte, name string, isZip bool) ([]byte, error) {
	if isZip {
		z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range z.File {
			if filepath.Base(f.Name) == name {
				r, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer r.Close()
				return io.ReadAll(r)
			}
		}
		return nil, errors.New("no " + name + " in it")
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	for t := tar.NewReader(gz); ; {
		h, err := t.Next()
		if err != nil {
			return nil, errors.New("no " + name + " in it")
		}
		if filepath.Base(h.Name) == name && h.Typeflag == tar.TypeReg {
			return io.ReadAll(t)
		}
	}
}

// kiriBin is where Kiri puts its commands (kiri, and the docker that talks to it).
func kiriBin() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kiri", "bin")
}

// docker is the docker command to run: the PATH's, else Kiri's.
func docker() string {
	if path, err := exec.LookPath("docker"); err == nil {
		return path
	}
	if path := filepath.Join(kiriBin(), exe("docker")); exists(path) {
		return path
	}
	return ""
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func up(docker string) bool { return docker != "" && exec.Command(docker, "info").Run() == nil }

// Containers makes sure this machine can run containers, and gives what a program that runs them needs in its
// environment (a PATH with docker on it). An engine that already answers is used as it is: CI's, a server's, a
// developer's own. Otherwise Kiri is started, and installed first where it is missing: with Homebrew on a Mac that
// has it, else with Kiri's install script (Mac and Linux), which checks what it downloads before running it.
func Containers() (env map[string]string, err error) {
	path := func() map[string]string {
		if _, err := exec.LookPath("docker"); err == nil {
			return nil
		}
		// For this command too, not only the programs it starts: it finds docker by name itself.
		with := kiriBin() + string(os.PathListSeparator) + os.Getenv("PATH")
		os.Setenv("PATH", with)
		return map[string]string{"PATH": with}
	}
	if up(docker()) {
		return path(), nil
	}
	kiri, _ := exec.LookPath("kiri")
	if kiri == "" && exists(filepath.Join(kiriBin(), exe("kiri"))) {
		kiri = filepath.Join(kiriBin(), exe("kiri"))
	}
	if kiri == "" && docker() != "" {
		return nil, errors.New("docker is installed and its engine does not answer (docker info): start it, then run this again")
	}
	if kiri == "" {
		if err := installKiri(); err != nil {
			return nil, err
		}
	}
	fmt.Println("starting Kiri, which runs the containers")
	switch runtime.GOOS {
	case "darwin":
		// Opening the app starts its engine (and, the first time, sets up its docker command).
		exec.Command("open", "-ga", "Kiri").Run()
	case "linux":
		exec.Command("systemctl", "--user", "start", "kiri").Run()
	}
	for range 90 {
		if up(docker()) {
			return path(), nil
		}
		time.Sleep(time.Second)
	}
	return nil, errors.New("Kiri did not start within 90 s: open it, and see its window (kiri ui) for why")
}

// installKiri installs Kiri the way its own guide says for this system (https://docs.runkiri.com/install).
func installKiri() error {
	run := func(name string, args ...string) error {
		fmt.Printf("$ %s %s\n", name, strings.Join(args, " "))
		cmd := exec.Command(name, args...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return cmd.Run()
	}
	fmt.Println("no container engine on this machine: installing Kiri (https://runkiri.com), which charter pack runs Fern's generators in")
	switch runtime.GOOS {
	case "darwin":
		if brew, err := exec.LookPath("brew"); err == nil {
			return run(brew, "install", "--cask", "libracoder/kiri/kiri")
		}
		return run("bash", "-c", "curl -fsSL https://runkiri.com/install.sh | bash")
	case "linux":
		return run("bash", "-c", "curl -fsSL https://runkiri.com/install.sh | bash")
	default:
		return errors.New("install Kiri for Windows from https://runkiri.com/download (its installer needs no administrator), then run this again")
	}
}

// State says what this machine has of the two, for charter env.
func State() (fnox, containers string) {
	cache, _ := os.UserCacheDir()
	if bin := filepath.Join(cache, "charter", "tools", "fnox-"+fnoxVersion, exe("fnox")); exists(bin) {
		fnox = "fnox " + fnoxVersion + " (charter's: " + bin + ")"
	} else {
		fnox = "not yet: charter install brings fnox " + fnoxVersion
	}
	switch d := docker(); {
	case up(d):
		containers = "running (" + d + ")"
	case d != "" || exists(filepath.Join(kiriBin(), exe("kiri"))):
		containers = "installed, not running: charter pack starts it"
	default:
		containers = "not yet: charter install brings Kiri"
	}
	return fnox, containers
}
