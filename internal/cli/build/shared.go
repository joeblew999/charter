package build

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joeblew999/charter/internal/previewbuild"
)

// The build cache CI shares through the repository's GitHub releases: a Wasm built from exactly the same inputs on an
// earlier run (the commit a release is made of, a change to docs only) is fetched there instead of compiled again,
// which saves the minutes TinyGo and wasm-opt take. go tool charter ci run turns it on, on GitHub, by setting
// CHARTER_CACHE_REPO; storing needs GITHUB_TOKEN's write permission (a fork's run only fetches).
const sharedTag, sharedKeep = "charter-cache", 40

// sharedURL is GitHub's base URL for its API, uploads and downloads alike, in a test ("" is GitHub's own).
var sharedURL string

func shared() *previewbuild.GitHub {
	repo := os.Getenv("CHARTER_CACHE_REPO")
	if repo == "" {
		return nil
	}
	return &previewbuild.GitHub{Repo: repo, Token: os.Getenv("GITHUB_TOKEN"), Title: "Build cache",
		API: sharedURL, Uploads: sharedURL, Downloads: sharedURL,
		About: "Wasm builds by their inputs, for go tool charter build on CI (charter). Not a release.",
		Logf:  func(format string, args ...any) { fmt.Printf(format+"\n", args...) }}
}

// fetchShared puts the shared cache's build of key at cached, when it has one.
func fetchShared(key, cached string) {
	g := shared()
	if g == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	data, ok, err := g.Fetch(ctx, sharedTag, "wasm-"+key, 0, time.Second)
	if err != nil || !ok {
		return
	}
	if err := os.WriteFile(cached, data, 0o644); err == nil {
		fmt.Printf("build/app.wasm: from the repository's build cache (%s)\n", g.Repo)
	}
}

// storeShared adds the build at wasm to the shared cache as key's. A failure costs only the next run's time.
func storeShared(key, wasm string) {
	g := shared()
	if g == nil || g.Token == "" {
		return
	}
	data, err := os.ReadFile(wasm)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := g.Publish(ctx, sharedTag, "wasm-"+key, data, sharedKeep); err != nil {
		fmt.Printf("· the build cache: %v\n", err)
	}
}
