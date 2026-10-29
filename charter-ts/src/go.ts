// A Go project's parts of the commands: gofmt and go vet, gsx's committed output kept fresh, the Worker built with
// TinyGo tuned for Workers, and the Go SDK committed in sdk/go, made from the specs it says.
import { existsSync, lstatSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import type { GoConfig } from "./config.ts";
import { miseTool } from "./pins.ts";
import { output, run } from "./run.ts";
import { sdk, specsHash } from "./sdk.ts";
import { freeBundleLimit, made } from "./sizes.ts";

/** gofmt over paths in dir, then go vet for the host and, for the packages wasm names, for Wasm
 * (GOOS=js GOARCH=wasm), as a Worker is built. */
function vetted(dir: string, paths: string[], wasm: string, env: Record<string, string> = {}): void {
	const unformatted = output("gofmt", ["-l", ...paths], dir).trim();
	if (unformatted) throw new Error(`not formatted in ${dir} (gofmt -w writes them as they should be):\n${unformatted}`);
	run("go", ["vet", "./..."], env, dir);
	run("go", ["vet", wasm], { ...env, GOOS: "js", GOARCH: "wasm" }, dir);
}

/** goSpecProgram writes the program that writes a Go project's specs, and returns its package: the
 * contract's API (api.API in the project's module) given to charter/specs, which first stops on what
 * the specs don't show (specs.Problems), then writes both specs or, with -check, compares them. It is
 * the same in every project, so charter-ts writes it, into build/ (charter's); go vet and go test
 * leave out a folder whose name starts with _. */
export function goSpecProgram(): string {
	const module = /^module\s+(\S+)/m.exec(readFileSync("go.mod", "utf8"))?.[1];
	if (!module) throw new Error("go.mod names no module");
	const dir = join("build", "_spec");
	mkdirSync(dir, { recursive: true });
	writeFileSync(join(dir, "main.go"), `// Written by charter-ts spec, each time it runs: don't edit. The program that writes the API's
// specs (go run ./build/_spec [-check] <openapi.json> <asyncapi.json> <server>).
package main

import (
	"${module}/api"
	"github.com/joeblew999/charter/specs"
)

func main() {
	spec := api.API(api.Env{})
	specs.Must(specs.Problems(spec))
	specs.Main(spec.OpenAPI, spec.AsyncAPI)
}
`);
	return `./build/_spec`;
}

/** A project's lint: its root's Go files and the config's folders vetted (the root package, the
 * Worker, for Wasm too), gsx's output and the committed Go SDK fresh. */
export function goLint(config: GoConfig): void {
	vetted(".", [...readdirSync(".").filter(name => name.endsWith(".go")), ...(config.lint ?? ["api"]).filter(dir => existsSync(dir))], ".");
	if (config.gsx) gsxCheck();
	if (config.sdk) sdkFresh();
}

/** goCheck is a Go module's check, for one that is not a project (charter's own, at the repo's top): its
 * packages' files vetted, for Wasm too, then go test. Only the module's own packages (GOWORK=off): the
 * examples beside them are modules of their own in the workspace, each with a check of its own. */
export function goCheck(dir: string): void {
	const env = { GOWORK: "off" };
	const packages = output("go", ["list", "-e", "-f", "{{.Dir}}", "./..."], dir, env).trim().split("\n").filter(Boolean);
	const files = packages.flatMap(pkg => readdirSync(pkg).filter(name => name.endsWith(".go")).map(name => relative(resolve(dir), join(pkg, name))));
	vetted(dir, files, "./...", env);
	run("go", ["test", "./..."], env, dir);
}

/** Not looked in for generated files: what is installed or built. */
const notSources = new Set(["node_modules", "build", "dist", "target", "out"]);

function snapshot(dir: string, match: (name: string) => boolean, into = new Map<string, string>()): Map<string, string> {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const path = dir === "." ? entry.name : join(dir, entry.name);
		if (entry.isDirectory()) {
			if (!entry.name.startsWith(".") && !notSources.has(entry.name)) snapshot(path, match, into);
		} else if (match(entry.name)) into.set(path, readFileSync(path, "utf8"));
	}
	return into;
}

/** gsxCheck runs gsx's generator and fails, naming them, if it changed a committed *.x.go: they were
 * stale. What it wrote stays, so what to commit is there. */
function gsxCheck(): void {
	const isGenerated = (name: string) => name.endsWith(".x.go");
	const before = snapshot(".", isGenerated);
	run("go", ["tool", "gsx", "generate", "--no-cache", "-q"]);
	const after = snapshot(".", isGenerated);
	const changed = [
		...[...after].filter(([file, content]) => before.get(file) !== content).map(([file]) => (before.has(file) ? file : `${file} (new)`)),
		...[...before.keys()].filter(file => !after.has(file)).map(file => `${file} (removed)`),
	].sort();
	if (changed.length > 0) throw new Error(`gsx changed what it generates, so the committed files were stale. They are written now: commit them.\n  ${changed.join("\n  ")}`);
	console.log("gsx: the committed *.x.go are what the components generate");
}

// The TinyGo the build uses, and the wasm-opt it runs, at the versions mise.toml pins (pins.ts): the
// repo's own, else charter's. The build patches TinyGo's runtime (internal/cli/build), and
// fails, saying so, when a TinyGo no longer has what a patch changes. mise installs them the first time.
/** tinygoEnv runs a command with the pinned TinyGo and wasm-opt on the PATH. */
const tinygoEnv = (...command: string[]): [string, string[]] => ["mise", ["exec", miseTool("tinygo"), miseTool("aqua:WebAssembly/binaryen"), "--", ...command]];

/** goBuild builds the project's Go Worker into build/ with charter's build step (TinyGo with the
 * runtime patches, a starting heap, a stack per goroutine Huma fits in, the Worker glue, and a cache of
 * Wasm made before: internal/cli/build has why), with the TinyGo and wasm-opt charter pins on the
 * PATH, and fails over the size Workers Free allows. It is charter build, the step a Go repo without
 * charter-ts runs from mise, so both build the same Wasm; here run as the tool of the project's go.mod (tool
 * github.com/joeblew999/charter/cmd/charter), until the projects take charter from mise too (#121). */
export function goBuild(config: GoConfig, limit = freeBundleLimit, fresh = false): void {
	const { heapMB = 8, opt = "z", stack = "128kb" } = config.build ?? {};
	const [program, args] = tinygoEnv("go", "tool", "charter", "build", `-heap=${heapMB}`, `-opt=${opt}`, `-stack=${stack}`, ...(fresh ? ["-fresh"] : []));
	run(program, args);
	if (made(join("build", "app.wasm"), "the Worker's Wasm", limit) > limit) throw new Error("build/app.wasm is over the limit (charter.config.ts: bundleLimit)");
}

const madeFrom = ".made-from";
const sdkDir = join("sdk", "go");

/** sdkFresh fails if the specs changed since sdk/go was made from them (no Docker: it compares a hash). */
function sdkFresh(): void {
	if (!existsSync(sdkDir)) return;
	const made = existsSync(join(sdkDir, madeFrom)) ? readFileSync(join(sdkDir, madeFrom), "utf8").trim() : "";
	if (made !== specsHash()) throw new Error(`${sdkDir} was not made from the specs in fern/ as they are now: npm run sdk -- --publish writes it again (Docker), then commit it`);
	console.log(`${sdkDir} was made from the specs in fern/ as they are now`);
}

/** The files of a generated Go SDK that are committed: what a program that imports it needs and the
 * tests that run on their own; not Fern's run record (.fern), nor WireMock's fixtures and the tests
 * that need its container. */
function sdkSources(dir: string, into = new Map<string, Buffer>(), base = dir): Map<string, Buffer> {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const path = join(dir, entry.name);
		if (entry.isDirectory()) {
			if (entry.name !== ".fern" && entry.name !== "wiremock" && !entry.name.endsWith("_test")) sdkSources(path, into, base);
		} else if (lstatSync(path).isFile() && entry.name !== "CONTRIBUTING.md" && entry.name !== madeFrom) into.set(relative(base, path).replaceAll("\\", "/"), readFileSync(path));
	}
	return into;
}

/** sdkPublish generates the Go SDK (Docker), proves it (build, vet, Fern's tests against WireMock) and
 * copies its sources into sdk/go, the committed module another repo can go get. With check it writes
 * nothing and fails if sdk/go is not what the specs generate now. */
export async function sdkPublish(check: boolean): Promise<void> {
	const out = join("sdk", "out", "go");
	rmSync(out, { recursive: true, force: true });
	sdk("go");
	const module = /^module\s+(\S+)/m.exec(readFileSync(join(out, "go.mod"), "utf8"))?.[1];
	if (!module?.endsWith("/sdk/go")) throw new Error(`${out}/go.mod declares ${module}: npm run spec writes the go group's module as <the project's module>/sdk/go`);
	const generated = sdkSources(out);
	const readme = generated.get("README.md") ?? Buffer.from("");
	generated.set("README.md", Buffer.concat([Buffer.from("> **Generated code.** Fern generated this Go client from the API's contract (the specs in `fern/`). Don't edit it: change the contract, then `npm run sdk -- --publish` writes this folder again.\n\n"), readme]));
	if (check) {
		const committed = existsSync(sdkDir) ? sdkSources(sdkDir) : new Map<string, Buffer>();
		const stale = [
			...[...generated].filter(([file, content]) => !committed.get(file)?.equals(content)).map(([file]) => `  ${committed.has(file) ? "differs" : "missing"}: ${file}`),
			...[...committed.keys()].filter(file => !generated.has(file)).map(file => `  not generated: ${file}`),
		].sort();
		if (stale.length > 0) throw new Error(`${sdkDir} is stale against the specs in fern/ (npm run sdk -- --publish writes it again):\n${stale.join("\n")}`);
		console.log(`${sdkDir} is what the specs in fern/ generate (${generated.size} files)`);
		return;
	}
	// Proven before it is committed, as sdk --prove proves it.
	await (await import("./sdk.ts")).proveGroup("go");
	rmSync(sdkDir, { recursive: true, force: true });
	for (const [file, content] of generated) {
		mkdirSync(join(sdkDir, file, ".."), { recursive: true });
		writeFileSync(join(sdkDir, file), content);
	}
	writeFileSync(join(sdkDir, madeFrom), `${specsHash()}\n`);
	console.log(`published: ${sdkDir} (module ${module}, ${generated.size} files). Commit it; another repo gets it with: go get ${module}@<branch, commit or version>`);
}
