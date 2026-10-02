package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
)

// The TypeScript SDK on Cloudflare Workers: a project whose Worker imports its own generated SDK
// (src/client) and tests it from inside workerd, as this repo's showcase-ts example does. The Worker
// serves the API itself (/api/mock/*), and /api/sdk-test runs the generated SDK against it:
// pagination, OAuth, idempotency, SSE, file upload, webhook signatures, WebSockets.

func init() {
	commands["harness-sync"] = command{"", "copy the project's compiled TypeScript SDK (sdk/out/typescript-dist) into its Worker (src/client), generating it first if the specs are newer", harnessSync}
	commands["harness-test"] = command{"[-remote]", "run the SDK test inside the project's Worker: under cf dev, or the deployed one (API_URL, API_MOCK_URL)", harnessTest}
	commands["harness-deploy"] = command{"", "REMOTE: deploy the project's Worker twice: as itself and, --mode api, as <worker>-api", harnessDeploy}
}

func harnessSync([]string) error {
	// Generate when there is no SDK yet, or when a spec or Fern's settings changed after it was made:
	// the specs come from the contract, so a contract change must reach the SDK the test runs.
	const sdk = "sdk/out/typescript-dist/esm/index.mjs"
	stale := !exists(sdk)
	if made, err := os.Stat(sdk); err == nil {
		entries, err := os.ReadDir(fernDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if info, err := entry.Info(); err == nil && info.ModTime().After(made.ModTime()) {
				stale = true
			}
		}
	}
	if stale {
		if err := sdkGen([]string{"typescript-dist"}); err != nil {
			return err
		}
	}
	if err := os.RemoveAll("src/client"); err != nil {
		return err
	}
	if err := os.CopyFS("src/client", os.DirFS("sdk/out/typescript-dist/esm")); err != nil {
		return err
	}
	fmt.Println("SDK copied into src/client")
	return nil
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

func harnessDeploy([]string) error {
	if err := harnessSync(nil); err != nil {
		return err
	}
	for _, mode := range []string{"production", "api"} {
		if err := sh(".", npmBin("cf"), "deploy", "--mode", mode); err != nil {
			return err
		}
	}
	return nil
}
