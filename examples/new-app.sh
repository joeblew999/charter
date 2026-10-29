#!/usr/bin/env bash
# examples/new-app.sh: an outside Go app, as `charter init` sets one up, on this checkout's charter: charter comes from
# mise (this checkout's build, linked as mise's version local, as `mise use github:joeblew999/charter` takes a release),
# and the app gets the TinyGo its patch is for from charter (.config/mise/conf.d/charter.toml) with no version of its
# own; mise gives charter and that TinyGo the three ways an app builds (a task on a Mac or in a Claude cloud session;
# charter's CI workflow, `mise install` of [ci]'s tools; Workers Builds' launcher, mise with only the app's config); it
# builds; and `charter check` refuses a TinyGo line the app adds itself, and an app that doesn't pin charter.
# ci.config.ts runs it as the go-app job.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
compat=$(sed -n 's/^compatibility_date = "\([0-9-]*\)".*/\1/p' "$root/examples/start-go/charter.toml")
want=$(sed -n 's/^tinygo = "\(.*\)"/\1/p' "$root/internal/pins/mise.toml")
go_pin=$(sed -n 's/^go = "\(.*\)"/\1/p' "$root/mise.toml")
app=$(mktemp -d)/app
mkdir -p "$app" && cd "$app"
step() { printf '\n== %s\n' "$*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

step "this checkout's charter, as mise's github:joeblew999/charter@local"
bin=$(mktemp -d)
go build -C "$root" -o "$bin/charter" ./cmd/charter
mise link -f github:joeblew999/charter@local "$bin"

step "a new app on this charter ($root), TinyGo $want expected from charter"
cat > go.mod <<EOF
module example.com/app

go 1.27.1

require github.com/joeblew999/charter v0.0.0

replace github.com/joeblew999/charter => $root
EOF
cat > main.go <<'EOF'
package main

import (
	"net/http"

	"github.com/joeblew999/charter/transport"
)

func main() {
	transport.Run(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("hello")) }))
}
EOF
printf '[tools]\ngo = "%s"\n"github:joeblew999/charter" = "local"\n' "$go_pin" > mise.toml
git init -q . && GOWORK=off go mod tidy
export GOWORK=off MISE_TRUSTED_CONFIG_PATHS="$app" MISE_YES=1
mise exec -- charter version
mise exec -- charter init -name app -compat "$compat"
[ -f .config/mise/conf.d/charter.toml ] || fail "init wrote no .config/mise/conf.d/charter.toml"
cmp -s .config/mise/conf.d/charter.toml "$root/internal/pins/mise.toml" || fail "the app's conf.d/charter.toml is not charter's pins"

step "1. a Mac or a Claude cloud session: mise install, then the task"
mise install
mise exec -- tinygo version | grep -q "tinygo version $want " || fail "mise gave $(mise exec -- tinygo version), not $want"
mise exec -- charter check
mise run charter:build
[ -s build/app.wasm ] || fail "no build/app.wasm"
echo "build/app.wasm: $(wc -c < build/app.wasm) bytes, gzipped $(gzip -9 -c build/app.wasm | wc -c)"

step "2. charter's CI workflow: mise install of go and charter, then [ci]'s tools by name"
mise install go github:joeblew999/charter tinygo
mise which tinygo | grep -q "/$want/" || fail "mise install tinygo resolved $(mise which tinygo)"
mise which charter | grep -q "/local/" || fail "mise install github:joeblew999/charter resolved $(mise which charter)"

step "3. Workers Builds' launcher: mise with only the app's config"
empty=$(mktemp -d)/empty.toml # an empty config, named .toml; mktemp -d works with BSD's mktemp (a Mac) and GNU's
: > "$empty"
MISE_GLOBAL_CONFIG_FILE=$empty MISE_SYSTEM_CONFIG_FILE=$empty MISE_CEILING_PATHS="$(dirname "$app")" \
	mise exec -- tinygo version | grep -q "tinygo version $want " || fail "the launcher's mise did not give TinyGo $want"
MISE_GLOBAL_CONFIG_FILE=$empty MISE_SYSTEM_CONFIG_FILE=$empty MISE_CEILING_PATHS="$(dirname "$app")" \
	mise exec -- charter version || fail "the launcher's mise did not give charter"

step "an app's own TinyGo line is refused"
cp mise.toml mise.toml.kept
printf 'tinygo = "0.43.0"\n' >> mise.toml
if out=$(mise exec -- charter check 2>&1); then fail "check passed with the app's own tinygo line"; fi
echo "$out" | grep -q "mise.toml pins tinygo: delete that line" || fail "check said: $out"
echo "$out" | grep "pins tinygo"

step "an app on a release of charter's library that doesn't pin charter is told to"
# This app is on this checkout (a replace in go.mod), which needs no pin: without the replace, it is on a release.
printf '[tools]\ngo = "%s"\n' "$go_pin" > mise.toml
cp go.mod go.mod.kept
grep -v '^replace ' go.mod.kept > go.mod
if out=$(PATH="$bin:$PATH" charter check 2>&1); then fail "check passed with no charter in mise.toml"; fi
echo "$out" | grep -q "mise.toml doesn't pin charter: mise use github:joeblew999/charter@<version>" || fail "check said: $out"
echo "$out" | grep "pin charter"
mv go.mod.kept go.mod
mv mise.toml.kept mise.toml

step "pass: charter and TinyGo $want reach the app from mise, every way it builds"
