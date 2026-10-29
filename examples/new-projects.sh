#!/usr/bin/env bash
# examples/new-projects.sh: what a user gets from charter new, in TypeScript and in Go, from each example: a project
# made from an archive of this checkout's files (CHARTER_SOURCE, as charter new fetches a release's tag from GitHub),
# with charter-lib packed as a release packs it, installed, taking this checkout's charter from mise (mise use
# github:joeblew999/charter, as mise's version local), and its own check passing. A Go project takes this checkout's
# Go library, as it would the release's. charter.toml runs it as the new job (slow: it installs).
#   bash examples/new-projects.sh [start notes go-start go-notes]
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
templates=${*:-start notes go-start go-notes}
scratch=$(mktemp -d) # mktemp -d works with BSD's mktemp (a Mac) and GNU's
trap 'rm -rf "$scratch"' EXIT
step() { printf '\n== %s\n' "$*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

step "charter-lib, as a release packs it; this checkout's files, as GitHub's archive of a tag has them"
(cd "$root" && npm pack -w ts --pack-destination "$scratch" >/dev/null)
lib=$(ls "$scratch"/charter-lib-*.tgz)
(cd "$root" && git ls-files -co --exclude-standard) | while IFS= read -r f; do [ -e "$root/$f" ] && printf '%s\n' "$f"; done > "$scratch/files.txt"
tar -czf "$scratch/charter.tar.gz" -C "$root" -T "$scratch/files.txt"

step "this checkout's charter, as mise's github:joeblew999/charter@local"
mkdir -p "$scratch/bin"
go build -C "$root" -o "$scratch/bin/charter" ./cmd/charter
mise link -f github:joeblew999/charter@local "$scratch/bin"

for template in $templates; do
	name="my-$template"
	into="$scratch/$name"
	from=${template#go-}
	go=()
	[ "$from" = "$template" ] || go=(-go)
	step "charter new $name -from $from ${go[*]:-}"
	# From the scratch folder: no charter repo around it, so the examples are the archive's.
	# ${go[@]+…}: an empty array is unbound under set -u in bash 3.2 (a Mac's).
	(cd "$scratch" && CHARTER_SOURCE="$scratch/charter.tar.gz" "$scratch/bin/charter" new "$name" -from "$from" ${go[@]+"${go[@]}"} -into "$into")
	cd "$into"
	# The release's Go library is not tagged yet: this checkout's instead.
	if [ ${#go[@]} -gt 0 ]; then
		printf '\nreplace github.com/joeblew999/charter => %s\n' "$root" >> go.mod
		GOWORK=off go mod tidy
	fi
	# The release's charter-lib is not published yet: the one packed here instead (a TypeScript Worker's; a Go one's
	# package.json has only its tests' packages).
	if grep -q '"charter-lib"' package.json; then
		sed -i.bak "s#\"charter-lib\": \"[^\"]*\"#\"charter-lib\": \"file:$lib\"#" package.json && rm package.json.bak
		grep -q "file:$lib" package.json || fail "package.json's charter-lib is not the one packed here"
	fi
	npm install --no-audit --no-fund
	export MISE_TRUSTED_CONFIG_PATHS="$into" MISE_YES=1 GOWORK=off
	mise use github:joeblew999/charter@local
	mise exec -- charter check
	echo "PASS  a project made by charter new -from $from${go[*]:+ ${go[*]}} passes its check"
	cd "$scratch"
done
