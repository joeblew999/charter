// Package workerd is intl's Intl outside a Cloudflare Worker: workerd, the Workers runtime, serving intl's
// js/intl-worker.mjs with the app's catalogs, set as intl.Runtime. On a Worker, Intl is the Worker's own; anywhere
// else (an ordinary Go server, go test, charter intl) it is this one, so every place formats with the same engine.
//
// An app on an ordinary Go server starts it before serving:
//
//	if err := workerd.Start(); err != nil {
//		log.Fatal(err)
//	}
//
// workerd is the one npm installed for the app (Path). The first Start runs it on a free port, serving the
// files charter intl gen writes beside the catalogs (intl-workerd/), and it stops when the program that started it
// ends, however it ends. INTL_WORKERD_URL names a workerd of your own instead (a server's, deployed with it). Each Intl
// call is a request to it on 127.0.0.1, about 0.1 ms.
package workerd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/joeblew999/charter/internal/intlkit/js"
	"github.com/joeblew999/charter/site/intl"
	"github.com/joeblew999/charter/site/intl/config"
)

// Path is the workerd intl runs: INTL_WORKERD when it names one, else the one npm installed for the app, in the
// node_modules nearest the working directory (wrangler brings it, for macOS, Linux and Windows).
func Path() (string, error) {
	if p := os.Getenv("INTL_WORKERD"); p != "" {
		return p, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if p, ok := native(filepath.Join(dir, "node_modules")); ok {
			return p, nil
		}
		if filepath.Dir(dir) == dir {
			return "", errors.New("intl/workerd: no workerd in a node_modules at or above " + wd + ": npm install (wrangler, in package.json, brings it), or set INTL_WORKERD to one of your own")
		}
	}
}

// native is the workerd binary of npm's workerd package in node_modules: its platform's package beside it
// (@cloudflare/workerd-darwin-arm64, …), where npm, pnpm and aube put it, or the package's own bin/workerd, which its
// install script makes the binary itself outside Windows.
func native(nodeModules string) (string, bool) {
	pkg, err := filepath.EvalSymlinks(filepath.Join(nodeModules, "workerd"))
	if err != nil {
		return "", false
	}
	arch := map[string]string{"amd64": "64", "arm64": "arm64"}[runtime.GOARCH]
	exe := map[bool]string{true: ".exe"}[runtime.GOOS == "windows"]
	platform := filepath.Join("@cloudflare", "workerd-"+runtime.GOOS+"-"+arch, "bin", "workerd"+exe)
	for _, p := range []string{
		filepath.Join(filepath.Dir(pkg), platform),
		filepath.Join(nodeModules, platform),
		filepath.Join(nodeModules, ".aube", "node_modules", platform),
	} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, true
		}
	}
	if p := filepath.Join(pkg, "bin", "workerd"); runtime.GOOS != "windows" && isBinary(p) {
		return p, true
	}
	return "", false
}

// isBinary reports whether the file at p is a program of its own, not a script (a #! line).
func isBinary(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 2)
	n, _ := io.ReadFull(f, head)
	return n == 2 && string(head) != "#!"
}

var (
	base   string
	client = &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 32}, Timeout: 10 * time.Second}
)

// Start sets intl.Runtime to workerd's Intl: INTL_WORKERD_URL's, or else a workerd of this program's own, started
// here for the app whose charter.toml's [intl] is found up from the working directory.
func Start() error {
	if base = strings.TrimSuffix(os.Getenv("INTL_WORKERD_URL"), "/"); base == "" {
		var err error
		if base, err = serve(); err != nil {
			return err
		}
	}
	// As on a Worker, an Intl that can't answer gives "" (the value's machine form shows); the reason goes to stderr.
	intl.Runtime = func(method string, args ...string) string {
		s, err := post(map[string]any{"method": method, "args": args})
		if err != nil {
			fmt.Fprintln(os.Stderr, "intl/workerd:", method, err)
			return ""
		}
		return s
	}
	return nil
}

// served is the workerd config to serve: the one charter intl gen wrote beside the app's catalogs (intl-workerd/), for
// the app whose charter.toml's [intl] is found up from the working directory; with no charter.toml's [intl], intl's own modules with no
// catalogs, in charter's cache: Intl's formats with no messages, which is all a program without catalogs asks for.
func served() (string, error) {
	c, err := config.Find(".")
	if err == nil {
		capnp := filepath.Join(c.CatalogDir(), "intl-workerd", "intl-workerd.capnp")
		if _, err := os.Stat(capnp); err != nil {
			return "", fmt.Errorf("%s: %w (charter intl gen writes it)", capnp, err)
		}
		return capnp, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "charter", "intl-workerd")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	files := map[string][]byte{"intl-messages.mjs": []byte("export default {};\n")}
	for _, name := range js.Workerd {
		b, err := js.FS.ReadFile(name)
		if err != nil {
			return "", err
		}
		files[name] = b
	}
	for name, b := range files {
		// Written only when it differs: several test programs start at once, and workerd may be reading one.
		if old, err := os.ReadFile(filepath.Join(dir, name)); err == nil && bytes.Equal(old, b) {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "intl-workerd.capnp"), nil
}

// keeper is the variable that makes this program workerd's keeper instead of itself: its value is workerd's config.
const keeper = "CHARTER_INTL_KEEPER"

// serve starts workerd for this program and returns its URL. workerd runs under a keeper, this same program started
// again (init), which holds workerd for as long as its stdin stays open: this program's end, of any kind, closes it,
// and the keeper stops workerd. So no workerd outlives the tests or the command that started it.
func serve() (string, error) {
	capnp, err := served()
	if err != nil {
		return "", err
	}
	if _, err := Path(); err != nil { // downloaded here, where the reader sees why it waits
		return "", err
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(self)
	cmd.Env = append(os.Environ(), keeper+"="+capnp)
	cmd.Stderr = os.Stderr
	if _, err := cmd.StdinPipe(); err != nil { // held open until this program ends
		return "", err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("intl/workerd: workerd didn't start: %w", err)
	}
	url := strings.TrimSpace(line)
	for until := time.Now().Add(20 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return url, nil
		}
		if time.Now().After(until) {
			return "", fmt.Errorf("intl/workerd: workerd at %s didn't answer: %w", url, err)
		}
	}
}

// init makes this program workerd's keeper when it was started as one (serve).
func init() {
	capnp := os.Getenv(keeper)
	if capnp == "" {
		return
	}
	os.Exit(keep(capnp))
}

// keep runs workerd on a free port, prints its URL, and stops it when stdin closes.
func keep(capnp string) int {
	bin, err := Path()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// A free port, found and let go again: workerd then listens on it. (Handing workerd the open socket would close
	// the gap between the two, but Windows cannot pass one to a program it starts.)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	addr := ln.Addr().String()
	ln.Close()
	url := "http://" + addr
	cmd := exec.Command(bin, "serve", capnp, "--socket-addr", "http="+addr)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "intl/workerd:", err)
		return 1
	}
	fmt.Println(url)
	done := make(chan struct{})
	go func() { cmd.Wait(); close(done) }()
	go func() { io.Copy(io.Discard, os.Stdin); cmd.Process.Kill() }()
	<-done
	return 0
}

// Tree formats a message tree as charter intl gen writes it (js/intl.mjs message) in lang with args: for tests of
// the catalogs' compiler.
func Tree(lang string, tree json.RawMessage, args ...string) (string, error) {
	return post(map[string]any{"method": "tree", "args": append([]string{lang}, args...), "tree": tree})
}

func post(body map[string]any) (string, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	resp, err := client.Post(base, "application/json", bytes.NewReader(b))
	// The daemon restarting, after gen rewrote its files: it is back in a moment.
	for until := time.Now().Add(20 * time.Second); err != nil && time.Now().Before(until); {
		time.Sleep(100 * time.Millisecond)
		resp, err = client.Post(base, "application/json", bytes.NewReader(b))
	}
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("workerd: %d %s", resp.StatusCode, out)
	}
	return string(out), nil
}
