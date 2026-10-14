package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// charter perf: REMOTE: build, deploy to a scratch Worker <worker>-perf-<experiment>, bench it from its first request, delete it (-keep: leave it)
// Run by: mise run perf:try (tasks/repo/tasks.toml)
func init() {
	commands["perf"] = command{"-name <experiment> [-build '<wasm-build flags>'] [-prebuilt] [-keep] [-- <bench flags>]",
		"REMOTE: build, deploy to a scratch Worker <worker>-perf-<experiment>, bench it from its first request, delete it (-keep: leave it)", perf}
}

var experimentName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,20}[a-z0-9])?$`)

func perf(args []string) error {
	name, build, keep, prebuilt := "", "", false, false
	rest := flags("perf", args, func(f *flag.FlagSet) {
		f.StringVar(&name, "name", "", "the experiment: lower-case letters, digits and hyphens, at most 22. Its Worker is <worker>-perf-<name>")
		f.StringVar(&build, "build", "", "flags for wasm-build, e.g. '-plain' or '-heap 4 -stack 96kb'")
		f.BoolVar(&prebuilt, "prebuilt", false, "deploy the build/ that is there (made by another TinyGo, say) instead of building")
		f.BoolVar(&keep, "keep", false, "leave the scratch Worker and its database (perf-clean removes them later)")
	})
	if !experimentName.MatchString(name) {
		return errors.New("perf needs -name <experiment>: lower-case letters, digits and hyphens, e.g. -name stack96")
	}
	address, base, err := deployed()
	if err != nil {
		return fmt.Errorf("perf derives the scratch Worker's URL from the deployed one: %w", err)
	}
	_, domain, _ := strings.Cut(address.Host, ".")
	mode := "perf-" + name
	worker := base + "-" + mode
	scratch := "https://" + worker + "." + domain

	if !prebuilt {
		if err := wasmBuild(strings.Fields(build)); err != nil {
			return err
		}
	}
	if !keep {
		defer func() {
			if err := deleteScratch(worker); err != nil {
				fmt.Println("not cleaned up (charter perf-clean does it):", err)
			}
		}()
	}
	// The scratch Worker's secrets are throwaway tokens: the bench writes with WRITE_TOKEN's.
	throwaway := map[string]string{}
	for _, name := range workerSecretNames() {
		throwaway[name] = randomToken()
	}
	secrets, err := secretsFile(throwaway)
	if err != nil {
		return err
	}
	deployArgs := []string{"deploy", "--mode", mode}
	if secrets != "" {
		defer os.Remove(secrets)
		deployArgs = append(deployArgs, "--secrets-file", secrets)
	}
	if err := quiet(".", nil, npmBin("cf"), deployArgs...); err != nil {
		return err
	}
	if err := migrate([]string{"-worker", worker}); err != nil {
		return err
	}
	// A new workers.dev name takes a while to answer.
	fmt.Printf("deployed %s; waiting for it to answer...\n", scratch)
	for deadline := time.Now().Add(90 * time.Second); ; time.Sleep(2 * time.Second) {
		res, err := benchClient.Get(scratch + "/api/openapi.json")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not answer in 90 s", scratch)
		}
	}
	// Deployed again, so that the bench meets a new isolate: the requests above used the first.
	if err := quiet(".", nil, npmBin("cf"), deployArgs...); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	if len(rest) == 0 {
		rest = []string{"-each", "-burst", "8", "-write"}
	}
	if token := throwaway["WRITE_TOKEN"]; token != "" {
		rest = append([]string{"-header", "Authorization: Bearer " + token}, rest...)
	}
	return bench(append(rest, "-worker", worker, scratch))
}

// randomToken is a new token: 32 random bytes, in hex.
func randomToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
