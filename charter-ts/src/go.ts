// A Go project's steps: gofmt and go vet, gsx's committed output kept fresh, the Worker built with
// TinyGo tuned for Workers, and the Go SDK committed in sdk/go, made from the specs it says.
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFileSync, cpSync, existsSync, lstatSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, symlinkSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";
import type { GoConfig } from "./config.ts";
import { cacheFolder, made, output, run, sdk, specsHash } from "./scripts.ts";

/** lint: gofmt over the root's Go files and the config's folders, then go vet for the host and, for
 * the root package, for Wasm (GOOS=js GOARCH=wasm), where the Worker is built from. */
export function goLint(config: GoConfig): void {
	const paths = [...readdirSync(".").filter(name => name.endsWith(".go")), ...(config.lint ?? ["api", "cmd"]).filter(dir => existsSync(dir))];
	const unformatted = output("gofmt", ["-l", ...paths]).trim();
	if (unformatted) throw new Error(`not formatted (gofmt -w writes them as they should be):\n${unformatted}`);
	run("go", ["vet", "./..."]);
	run("go", ["vet", "."], { GOOS: "js", GOARCH: "wasm" });
	if (config.gsx) gsxCheck();
	if (config.sdk) sdkFresh();
}

/** goCheck is a Go module's check, for one that is not a project (go/lib): gofmt over every file,
 * go vet for the host and for Wasm (GOOS=js GOARCH=wasm, as a Worker builds it), then go test. */
export function goCheck(dir: string): void {
	const unformatted = output("gofmt", ["-l", "."], dir).trim();
	if (unformatted) throw new Error(`not formatted in ${dir} (gofmt -w writes them as they should be):\n${unformatted}`);
	run("go", ["vet", "./..."], {}, dir);
	run("go", ["vet", "./..."], { GOOS: "js", GOARCH: "wasm" }, dir);
	run("go", ["test", "./..."], {}, dir);
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
export function gsxCheck(): void {
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

// The TinyGo the build uses, and the wasm-opt it runs: the compiler and the patch below are one
// thing, so they are pinned here together. mise installs them the first time.
const tinygoVersion = "0.43.0", binaryenVersion = "133";
const tinygo = (...args: string[]): [string, string[]] => ["mise", ["exec", `tinygo@${tinygoVersion}`, `aqua:WebAssembly/binaryen@${binaryenVersion}`, "--", "tinygo", ...args]];

// The change made to TinyGo's runtime source (read at build time: the compiler is not rebuilt).
//
// No collection at every pause. TinyGo runs a full collection whenever the scheduler goes idle once 32
// finalizers were registered since the last one. Every JavaScript reference registers one, and a
// Worker goes idle at every await: measured on Cloudflare, 38 to 71 ms of CPU per request against 9 to
// 13 ms without it (docs/benchmarks.md). The collector still runs when the heap is full.
//
// (TinyGo 0.43.0 frees the stacks of finished goroutines itself, tinygo-org/tinygo#5801: the patches
// that made it reuse them are gone.)
// Upstream: tinygo-org/tinygo#5800 (when fixed: drop the patch)
const patches: { file: string; old: string; new: string; unless?: string }[] = [
	{ file: "src/runtime/gc_finalizer.go", old: "const finalizerGCThreshold = 32", new: "const finalizerGCThreshold = 0" },
];

/** tinygoRoot is a TinyGo root whose runtime has the patches: the installed one with its src/ copied
 * and the patched files changed, made once per TinyGo version and kept in the cache folder. What is not
 * src/ is a link to the installed one. */
function tinygoRoot(): string {
	const installed = output(...tinygo("env", "TINYGOROOT")).trim().split("\n").at(-1)!.trim();
	const version = output(...tinygo("version"));
	const key = createHash("sha256").update(version + patches.map(p => p.file + p.old + p.new).join("")).digest("hex").slice(0, 12);
	const root = join(cacheFolder(), `tinygo-${key}`);
	if (existsSync(join(root, ".patched"))) return root;
	rmSync(root, { recursive: true, force: true });
	mkdirSync(root, { recursive: true });
	for (const name of readdirSync(installed)) {
		const from = join(installed, name), to = join(root, name);
		if (name === "src") {
			// TinyGo wants src/ to be real folders, and the patches change files in it. On macOS a clone.
			if (process.platform !== "darwin" || spawnSync("cp", ["-c", "-R", from, to]).status !== 0) cpSync(from, to, { recursive: true });
		} else {
			try {
				symlinkSync(from, to);
			} catch {
				cpSync(from, to, { recursive: true }); // Windows lets only some users make links
			}
		}
	}
	for (const patch of patches) {
		const file = join(root, patch.file);
		const source = existsSync(file) ? readFileSync(file, "utf8") : "";
		if (patch.unless && source.includes(patch.unless)) continue; // this TinyGo has the fix itself
		const crlf = source.includes("\r\n");
		const [old, replacement] = crlf ? [patch.old.replaceAll("\n", "\r\n"), patch.new.replaceAll("\n", "\r\n")] : [patch.old, patch.new];
		if (source.split(old).length !== 2) {
			rmSync(root, { recursive: true, force: true });
			throw new Error(`this TinyGo (${version.trim()}) does not have the text a patch changes (${JSON.stringify(patch.old)} in ${patch.file}): see docs/upstream.md`);
		}
		rmSync(file); // a new file, not new content in the installed one's (a hard link on Windows)
		writeFileSync(file, source.replace(old, replacement));
	}
	writeFileSync(join(root, ".patched"), version);
	console.log(`TinyGo runtime patched once into ${root}`);
	return root;
}

/** goBuild builds the project's Go Worker into build/app.wasm with TinyGo, tuned for Workers, and
 * fills build/ with what runs it: workers-go's JavaScript and the Go library's Worker glue. The Wasm
 * has the runtime patches, a starting heap so an ordinary request never collects, a stack per
 * goroutine Huma fits in, and fails over the size Workers Free allows. */
export function goBuild(config: GoConfig, fresh = false): void {
	const { heapMB = 8, opt = "z", stack = "128kb", maxBytes = 3_000_000 } = config.build ?? {};
	run("go", ["run", "github.com/syumai/workers-go/cmd/workers-assets-gen", "-mode=tinygo"]);
	glue();
	// The same sources, pins and options make the same Wasm: one made before is copied, not built.
	const cached = join(cacheFolder("wasm"), `${buildKey(config)}.wasm`);
	if (!fresh && existsSync(cached)) {
		copyFileSync(cached, join("build", "app.wasm"));
		console.log("build/app.wasm: built before from the same sources (charter's cache); --fresh builds it again");
	} else {
		const target = join("build", "tinygo-target.json");
		writeFileSync(target, `{ "inherits": ["wasm"], "ldflags": ["--initial-memory=${heapMB * 1024 * 1024}"] }\n`);
		const [program, args] = tinygo("build", "-o", "build/app.wasm", "-target", target, "-no-debug", `-opt=${opt}`, `-stack-size=${stack}`, ".");
		run(program, args, { TINYGOROOT: tinygoRoot() });
		copyFileSync(join("build", "app.wasm"), cached);
	}
	if (made(join("build", "app.wasm"), "the Worker's Wasm", maxBytes) > maxBytes) throw new Error("build/app.wasm is over the limit");
}

/** buildKey names what goes into the Wasm: the Go version, TinyGo's pins and patches, the build's
 * options, go.mod and go.sum, and every package the Worker is built from that is not the standard
 * library's, with its Go files (whatever their build tags) and the files it embeds: nothing else in the
 * folder goes into the Wasm. A module from the module cache counts by its path, which has its
 * version: it never changes. */
function buildKey(config: GoConfig): string {
	const hash = createHash("sha256");
	hash.update(JSON.stringify({ go: output("go", ["version"]), tinygoVersion, binaryenVersion, patches, build: config.build ?? {} }));
	for (const name of ["go.mod", "go.sum"]) if (existsSync(name)) hash.update(readFileSync(name));
	const modcache = output("go", ["env", "GOMODCACHE"]).trim();
	const packages = output("go", ["list", "-deps", "-tags", "tinygo", "-f", '{{if not .Standard}}{{.Dir}}{{"\t"}}{{join .EmbedFiles " "}}{{end}}', "."], undefined, { GOOS: "js", GOARCH: "wasm" });
	for (const line of packages.split("\n").filter(Boolean).sort()) {
		const [dir, embeds = ""] = line.split("\t");
		if (dir.startsWith(modcache)) {
			hash.update(`${relative(modcache, dir)}\n`);
			continue;
		}
		hash.update(`${relative(".", dir)}\n`);
		const files = [...readdirSync(dir).filter(name => name.endsWith(".go") && statSync(join(dir, name)).isFile()), ...embeds.split(" ").filter(Boolean)].sort();
		for (const name of files) {
			hash.update(`${name}\n`);
			hash.update(readFileSync(join(dir, name)));
		}
	}
	return hash.digest("hex").slice(0, 24);
}

/** glue writes the Go library's Worker glue (worker/*.mjs) into build/, taken from the library as the
 * project's go.mod resolves it, so the JavaScript is always the one written for the Go it builds against. */
function glue(): void {
	const library = output("go", ["list", "-m", "-f", "{{.Dir}}", "github.com/joeblew999/charter/go"]).trim();
	const worker = join(library, "worker");
	const files = existsSync(worker) ? readdirSync(worker).filter(name => name.endsWith(".mjs")) : [];
	if (files.length === 0) throw new Error(`no Worker glue (worker/*.mjs) in ${library}: the project's go.mod requires github.com/joeblew999/charter/go`);
	mkdirSync("build", { recursive: true });
	for (const name of files) writeFileSync(join("build", name), readFileSync(join(worker, name)));
}

const madeFrom = ".made-from";
const sdkDir = join("sdk", "go");

/** sdkFresh fails if the specs changed since sdk/go was made from them (no Docker: it compares a hash). */
export function sdkFresh(): void {
	if (!existsSync(sdkDir)) return;
	const made = existsSync(join(sdkDir, madeFrom)) ? readFileSync(join(sdkDir, madeFrom), "utf8").trim() : "";
	if (made !== specsHash()) throw new Error(`${sdkDir} was not made from the specs in fern/ as they are now: npm run sdk:publish writes it again (Docker), then commit it`);
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
export function sdkPublish(check: boolean): void {
	const out = join("sdk", "out", "go");
	rmSync(out, { recursive: true, force: true });
	sdk("go");
	const module = /^module\s+(\S+)/m.exec(readFileSync(join(out, "go.mod"), "utf8"))?.[1];
	if (!module?.endsWith("/sdk/go")) throw new Error(`${out}/go.mod declares ${module}: set config.module.path of the go group in fern/generators.yml to <the project's module>/sdk/go`);
	const generated = sdkSources(out);
	const readme = generated.get("README.md") ?? Buffer.from("");
	generated.set("README.md", Buffer.concat([Buffer.from("> **Generated code.** Fern generated this Go client from the API's contract (the specs in `fern/`). Don't edit it: change the contract, then `npm run sdk:publish` writes this folder again.\n\n"), readme]));
	if (check) {
		const committed = existsSync(sdkDir) ? sdkSources(sdkDir) : new Map<string, Buffer>();
		const stale = [
			...[...generated].filter(([file, content]) => !committed.get(file)?.equals(content)).map(([file]) => `  ${committed.has(file) ? "differs" : "missing"}: ${file}`),
			...[...committed.keys()].filter(file => !generated.has(file)).map(file => `  not generated: ${file}`),
		].sort();
		if (stale.length > 0) throw new Error(`${sdkDir} is stale against the specs in fern/ (npm run sdk:publish writes it again):\n${stale.join("\n")}`);
		console.log(`${sdkDir} is what the specs in fern/ generate (${generated.size} files)`);
		return;
	}
	// Proven before it is committed. A generated module is its own: no workspace above it applies.
	run("go", ["build", "./..."], { GOWORK: "off" }, out);
	run("go", ["vet", "./..."], { GOWORK: "off" }, out);
	const compose = join(out, "wiremock", "docker-compose.test.yml");
	if (existsSync(compose)) {
		run("docker", ["compose", "-f", compose, "up", "-d", "--wait"]);
		try {
			const port = output("docker", ["compose", "-f", compose, "port", "wiremock", "8080"]).trim();
			run("go", ["test", "./..."], { GOWORK: "off", WIREMOCK_URL: `http://${port}` }, out);
		} finally {
			run("docker", ["compose", "-f", compose, "down"]);
		}
	}
	rmSync(sdkDir, { recursive: true, force: true });
	for (const [file, content] of generated) {
		mkdirSync(join(sdkDir, file, ".."), { recursive: true });
		writeFileSync(join(sdkDir, file), content);
	}
	writeFileSync(join(sdkDir, madeFrom), `${specsHash()}\n`);
	console.log(`published: ${sdkDir} (module ${module}, ${generated.size} files). Commit it; another repo gets it with: go get ${module}@<branch, commit or version>`);
}
