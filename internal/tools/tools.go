// Package tools finds the programs charter runs beside Go and Node, and brings the one mise does not:
//
//   - mise (https://mise.jdx.dev) installs Go, Node and fnox at the versions the app's mise.toml pins, on every
//     machine and in CI, and mise.lock says what each download must be. It is the one thing a developer installs.
//   - fnox (https://fnox.jdx.dev) keeps secrets: the Cloudflare token a deploy needs comes from the keychain or a
//     password manager, never from a file or the shell's history. charter runs wrangler under fnox exec. It is a
//     tool of mise.toml.
//   - Kiri (https://runkiri.com) runs containers, which Fern's SDK generators are: charter pack uses its docker.
//     Where another Docker engine already answers (CI's own, a server's), that one is used and Kiri is left alone.
//     It is an app of its own, not a tool mise installs, so charter brings it when it is needed.
package tools

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// NoMise is what is said where mise is missing: how to install it, for each system.
const NoMise = "charter's tools come from mise (https://mise.jdx.dev): install it once (macOS: brew install mise; " +
	"Linux: curl https://mise.run | sh; Windows: winget install jdx.mise), let your shell use it (mise activate), " +
	"then mise install in the app's folder"

// Mise is the path of mise, "" when this machine has none.
func Mise() string {
	path, _ := exec.LookPath("mise")
	return path
}

var fnox struct {
	once sync.Once
	path string
	err  error
}

// Fnox is the path of fnox: the one mise.toml pins for the folder charter runs in, as mise resolves it. A fnox
// that is only on the PATH (a shim, another install) is used where mise cannot say.
func Fnox() (string, error) {
	fnox.once.Do(func() {
		if mise := Mise(); mise != "" {
			out, err := exec.Command(mise, "which", "fnox").Output()
			if path := strings.TrimSpace(string(out)); err == nil && exists(path) {
				fnox.path = path
				return
			}
		}
		if path, err := exec.LookPath("fnox"); err == nil && !strings.Contains(filepath.ToSlash(path), "/shims/") {
			fnox.path = path
			return
		}
		if Mise() == "" {
			fnox.err = errors.New("fnox is not here. " + NoMise)
		} else {
			fnox.err = errors.New("fnox is not installed for this folder: mise install (mise.toml pins it)")
		}
	})
	return fnox.path, fnox.err
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

// State says what this machine has of fnox and of a container engine, for charter env.
func State() (fnoxSays, containers string) {
	if path, err := Fnox(); err == nil {
		out, _ := exec.Command(path, "--version").Output()
		fnoxSays = strings.TrimSpace(string(out)) + " (" + path + ")"
	} else {
		fnoxSays = "not yet: " + err.Error()
	}
	switch d := docker(); {
	case up(d):
		containers = "running (" + d + ")"
	case d != "" || exists(filepath.Join(kiriBin(), exe("kiri"))):
		containers = "installed, not running: charter pack starts it"
	default:
		containers = "not yet: charter install brings Kiri"
	}
	return fnoxSays, containers
}
