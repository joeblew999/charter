package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
)

// charter harness-test: run the SDK test inside the project's Worker: under cf dev, or the deployed one (API_URL, API_MOCK_URL)
// Run by: mise run live-test, mise run test:workerd (the examples' mise.toml)
func init() {
	commands["harness-test"] = command{"[-remote]", "run the SDK test inside the project's Worker: under cf dev, or the deployed one (API_URL, API_MOCK_URL)", harnessTest}
}

func harnessTest(args []string) error {
	var remote bool
	flags("harness-test", args, func(f *flag.FlagSet) {
		f.BoolVar(&remote, "remote", false, "test the deployed Worker (API_URL) instead of cf dev")
	})
	if err := harnessSync(nil); err != nil {
		return err
	}
	base, target := "", ""
	if remote {
		var err error
		if base, err = env("API_URL"); err != nil {
			return err
		}
		// On Cloudflare the in-Worker WebSocket client needs a second Worker (a Worker can't call itself: 1042).
		api, err := env("API_MOCK_URL")
		if err != nil {
			return err
		}
		target = "?target=" + api
	} else {
		port, err := freePort()
		if err != nil {
			return err
		}
		base = "http://localhost:" + port
		stop, err := server([]string{"cf", "dev"}, []string{"PORT=" + port}, base+"/")
		if err != nil {
			return err
		}
		defer stop()
	}
	fmt.Println("Target:", base)
	failed := false
	for _, path := range []string{"/api/sdk-test", "/api/ws-test" + target} {
		res, err := http.Get(base + path)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		var report struct {
			Passed  bool
			Results map[string]struct {
				OK     bool
				Detail any
			}
		}
		if err := json.Unmarshal(body, &report); err != nil {
			fmt.Printf("%s: %.500s\n", path, body)
			failed = true
			continue
		}
		names := make([]string, 0, len(report.Results))
		for name := range report.Results {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if result := report.Results[name]; result.OK {
				fmt.Println("PASS ", name)
			} else {
				fmt.Printf("FAIL  %s  %.300v\n", name, result.Detail)
			}
		}
		failed = failed || !report.Passed
	}
	if err := sh(".", "node", "test/ws-client.mjs", base); err != nil {
		failed = true
	}
	if failed {
		return fmt.Errorf("the SDK test failed against %s", base)
	}
	return nil
}

// freePort asks the system for a port nobody is using.
func freePort() (string, error) {
	ports, err := freePorts(1)
	if err != nil {
		return "", err
	}
	return ports[0], nil
}
