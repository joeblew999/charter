package fern

import (
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/joeblew999/charter/internal/run"
)

// Docker, readied for what charter runs in containers (Fern's generators): running, and, where the machine reaches
// the internet through a proxy on itself with its own CA (a Claude cloud session), able to reach npm from inside a
// container. On a Mac or a GitHub runner the containers have the internet as they are, and this changes nothing but
// checking Docker is up.

// cloudCA is where a Claude cloud session keeps the CA of its egress proxy.
const cloudCA = "/root/.ccr/ca-bundle.crt"

//go:embed cloud/docker
var cloudDocker []byte

// ProxyCA is the CA file of a proxy on this machine, when its internet goes through one: HTTPS_PROXY on a loopback
// address, and its CA in NODE_EXTRA_CA_CERTS or where a Claude cloud session keeps it; "" otherwise.
func ProxyCA(getenv func(string) string, exists func(string) bool) string {
	proxy := getenv("HTTPS_PROXY")
	if proxy == "" {
		proxy = getenv("https_proxy")
	}
	u, err := url.Parse(proxy)
	if proxy == "" || err != nil || !slices.Contains([]string{"127.0.0.1", "localhost", "::1"}, u.Hostname()) {
		return ""
	}
	for _, ca := range []string{getenv("NODE_EXTRA_CA_CERTS"), cloudCA} {
		if ca != "" && exists(ca) {
			return ca
		}
	}
	return ""
}

// ContainerEnv is the environment for a program that runs containers: with a proxy on this machine, charter's docker
// (cloud/docker, written into charter's cache) first on the PATH and the proxy's CA named; otherwise nothing.
func ContainerEnv() (map[string]string, error) {
	ca := ProxyCA(os.Getenv, func(p string) bool { _, err := os.Stat(p); return err == nil })
	if ca == "" {
		return nil, nil
	}
	dir, err := run.Cache("cloud")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "docker"), cloudDocker, 0o755); err != nil {
		return nil, err
	}
	return map[string]string{"PATH": dir + string(os.PathListSeparator) + os.Getenv("PATH"), "CHARTER_PROXY_CA": ca}, nil
}

func dockerUp() bool { return exec.Command("docker", "info").Run() == nil }

// Ready makes sure Docker answers before a container is run. Where it is installed and not running and this user may
// start it (a cloud session, as root), it starts the daemon and waits; on a desktop it says to start Docker, rather
// than let a generator fail halfway.
func Ready() error {
	if dockerUp() {
		return nil
	}
	if dockerd, err := exec.LookPath("dockerd"); err == nil && os.Geteuid() == 0 {
		fmt.Printf("Docker is not running: starting %s (log: /tmp/charter-dockerd.log)\n", dockerd)
		log, err := os.OpenFile("/tmp/charter-dockerd.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		cmd := exec.Command(dockerd)
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			return err
		}
		cmd.Process.Release()
		for range 60 {
			if dockerUp() {
				return nil
			}
			time.Sleep(time.Second)
		}
		return errors.New("Docker did not start within 60 s: /tmp/charter-dockerd.log says why")
	}
	return errors.New("Docker is not running, and Fern's generators run in it: start Docker (Docker Desktop or OrbStack on a Mac), then run this again")
}
