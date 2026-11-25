package seo

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// showVersion prints this build's version (seo:show-version): the release mise installed, from the repo's pin in
// .mise/config.toml.
func showVersion(args []string) {
	parse(flags("show-version"), args, 0)
	fmt.Println(buildVersion())
}

// version is set by the release build (goreleaser's -X main.version).
var version = ""

// buildVersion is this build's version: goreleaser's for a release; otherwise Go's module version, or "(devel)". Only
// a release (version set) pins itself: any other build is a checkout's (mise run tool:install links it as local).
func buildVersion() string {
	if version != "" {
		return strings.TrimPrefix(version, "v")
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return "(devel)"
}
