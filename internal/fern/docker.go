package fern

import (
	_ "embed"
	"github.com/joeblew999/charter/internal/tools"
	"net/url"
	"os"
	"path/filepath"
	"slices"

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

// Ready makes sure containers can run before a generator is started, and gives what its environment needs: an
// engine that already answers is used (CI's own Docker), otherwise Kiri is started, and installed where it is missing
// (package tools).
func Ready() (map[string]string, error) { return tools.Containers() }
