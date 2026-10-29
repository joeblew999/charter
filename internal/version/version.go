// Package version is the version of charter a binary is: the release's, written in by its build (goreleaser),
// else the one Go records (go install …@v1.2.3, go tool charter in an app), else a build from a checkout's.
package version

import (
	"regexp"
	"runtime/debug"
)

// set is a release's version, written in by its build: -X github.com/joeblew999/charter/internal/version.set=v1.2.3.
var set string

// String is the version: v1.2.3 for a release, a pseudo-version or (devel) for a build from a checkout.
func String() string {
	if set != "" {
		return set
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

var release = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// Released reports whether this is a release (v1.2.3), the version a go.mod requires of the library: not a build
// from a checkout, whose version is a pseudo-version or (devel).
func Released() bool { return release.MatchString(String()) }
