// The SDKs, made with Fern from a project's fern/ (its specs and generators.yml): a group generated
// (Docker) into sdk/out/<group> when fern/ is newer than what made it, copied into a Worker that runs it,
// or proven in its language (sdk --prove). Go: build, vet, and Fern's tests against WireMock.
// TypeScript: typechecked with the TypeScript its SDKs are written for. Python: mypy and its own tests.
// The CLI (Rust): built. A language this does not know is generated and named, not proven. The
// toolchains a proof needs are mise's, at the versions pinned here, in mise's own folder.
import { createHash } from "node:crypto";
import { cpSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, symlinkSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { made } from "./sizes.ts";
import { cacheFolder, output, run, runQuietly, seconds } from "./run.ts";

/** The hash of what an SDK is made from: every file in fern/. */
export function specsHash(): string {
	const hash = createHash("sha256");
	for (const name of readdirSync("fern").sort()) {
		const file = join("fern", name);
		if (!statSync(file).isFile()) continue;
		const content = readFileSync(file);
		hash.update(`${name} ${content.length}\n`);
		hash.update(content);
	}
	return `sha256:${hash.digest("hex")} fern`;
}

/** sdk generates a group of fern/generators.yml with Fern (Docker) into sdk/out/<group>, and records
 * the hash of fern/ it was made from beside it. With ifStale, only when there is no SDK yet or fern/
 * (the specs, Fern's settings) is not what it was made from: the specs come from the contract, so a
 * contract change reaches the SDK. */
export function sdk(group: string, options: { ifStale?: boolean } = {}): void {
	if (options.ifStale && upToDate(group)) {
		console.log(`sdk ${group}: up to date (made from fern/ as it is), not generated again`);
		return;
	}
	run("fern", fernGenerate(group));
	stampMade(group);
}

/** generateQuietly generates a group as sdk does, with Fern's output kept rather than shown, so that
 * several can run at once (each in a Docker container of its own) without their lines mixing. It
 * resolves with how long it took, and rejects with Fern's output if it fails. */
export async function generateQuietly(group: string): Promise<number> {
	const { ms } = await runQuietly("fern", fernGenerate(group));
	stampMade(group);
	return ms;
}

const fernGenerate = (group: string) => ["generate", "--local", "--group", group, "--force", "--log-level", "warn"];
/** What each SDK in sdk/out was made from (specsHash, by group), in one file beside them. */
const madeFrom = join("sdk", "out", ".made-from.json");
const stamps = (): Record<string, string> => (existsSync(madeFrom) ? (JSON.parse(readFileSync(madeFrom, "utf8")) as Record<string, string>) : {});
const upToDate = (group: string) => existsSync(join("sdk", "out", group)) && stamps()[group] === specsHash();
function stampMade(group: string): void {
	mkdirSync(join("sdk", "out"), { recursive: true });
	writeFileSync(madeFrom, JSON.stringify({ ...stamps(), [group]: specsHash() }, null, "\t") + "\n");
}

/** syncSdk copies the TypeScript SDK (typescript-dist) into folder, for a Worker that runs it, generating
 * it first if the specs are newer. */
export function syncSdk(folder: string): void {
	sdk("typescript-dist", { ifStale: true });
	rmSync(folder, { recursive: true, force: true });
	cpSync(join("sdk", "out", "typescript-dist", "esm"), folder, { recursive: true });
	console.log(`SDK copied into ${folder}`);
}

/** The Rust the CLI is built with: mise loads it when a CLI is built, as it loads TinyGo. */
const rustVersion = "1.98.1";

/** What a Python client is proven with: uv (from mise) runs this Python with these packages, the
 * client's own (httpx, pydantic, typing-extensions, anyio) and the checkers. */
const uvVersion = "0.13.0", pythonVersion = "3.13";
const pythonPackages = ["httpx==0.28.1", "pydantic==2.14.1", "typing-extensions==4.16.0", "anyio==4.15.1", "pytest==9.1.1", "mypy==2.4.0"];

/** python proves a Python client: Fern writes the package's source alone, so it goes on an import
 * path under the name its tests import it by; then mypy over it, and its own tests. */
async function python(dir: string, group: string): Promise<void> {
	const tests = join(dir, "tests");
	const testText = existsSync(tests) ? readdirSync(tests).filter(name => name.endsWith(".py")).map(name => readFileSync(join(tests, name), "utf8")).join("\n") : "";
	const name = /^\s*from ([a-z_][a-z0-9_]*)\.client import/m.exec(testText)?.[1] ?? "client";
	const site = join("sdk", "out", ".work", `${group}-import`);
	rmSync(site, { recursive: true, force: true });
	mkdirSync(site, { recursive: true });
	symlinkSync(resolve(dir), join(site, name), "junction");
	const uv = (...args: string[]) => runQuietly("mise", ["exec", `uv@${uvVersion}`, "--", "uv", "run", "--no-project", "--python", pythonVersion, ...pythonPackages.flatMap(p => ["--with", p]), "--", ...args], {}, site);
	try {
		await uv("mypy", "-p", name);
		if (testText) await uv("python", "-m", "pytest", "-q", join(name, "tests"));
		else await uv("python", "-c", `import ${name}`);
	} finally {
		rmSync(site, { recursive: true, force: true });
	}
}

/** groups are the groups fern/generators.yml declares, in its order. */
export function groups(): string[] {
	const text = readFileSync(join("fern", "generators.yml"), "utf8");
	const block = text.split(/^groups:\s*$/m)[1] ?? "";
	return [...block.split(/^\S/m)[0].matchAll(/^ {2}([a-z][a-z0-9-]*):\s*$/gm)].map(m => m[1]);
}

// How a generated TypeScript SDK is typechecked: standard fetch, none of the project's own settings.
// The projects stay on @types/node 22: with 26, a generated TypeScript SDK's Headers does not typecheck.
// Upstream: fern-api/fern#18313 (when fixed: @types/node 26 in every package.json)
const tsconfig = (entry: string) => JSON.stringify({
	"//": "Written by charter-ts sdk --prove: how a generated TypeScript SDK is typechecked (bundler mode, default libs).",
	compilerOptions: { target: "es2022", module: "preserve", moduleResolution: "bundler", allowImportingTsExtensions: true, strict: true, noEmit: true, skipLibCheck: true },
	include: [entry],
}, null, 2);

/** proveGroup proves a generated group: in its language, by what is in sdk/out/<group>. What it runs keeps
 * its output, shown only if it fails: the groups are proven side by side. */
export async function proveGroup(group: string): Promise<void> {
	const dir = join("sdk", "out", group);
	if (!existsSync(dir) || readdirSync(dir).length === 0) throw new Error(`${dir}: nothing generated (charter-ts sdk ${group})`);
	if (existsSync(join(dir, "go.mod"))) {
		// A generated module is its own: no workspace above it applies.
		await runQuietly("go", ["build", "./..."], { GOWORK: "off" }, dir);
		await runQuietly("go", ["vet", "./..."], { GOWORK: "off" }, dir);
		const compose = join(dir, "wiremock", "docker-compose.test.yml");
		if (!existsSync(compose)) {
			await runQuietly("go", ["test", "./..."], { GOWORK: "off" }, dir);
			return;
		}
		await runQuietly("docker", ["compose", "-f", compose, "up", "-d", "--wait"]);
		try {
			const port = output("docker", ["compose", "-f", compose, "port", "wiremock", "8080"]).trim();
			await runQuietly("go", ["test", "./..."], { GOWORK: "off", WIREMOCK_URL: `http://${port}` }, dir);
		} finally {
			await runQuietly("docker", ["compose", "-f", compose, "down"]);
		}
		return;
	}
	const entry = ["index.ts", "src/index.ts", "sdk/index.ts"].find(name => existsSync(join(dir, name)));
	if (entry) {
		// Fern's SDKs are written for TypeScript 5: a project on a newer one has 5.9 beside it as
		// typescript-sdk ("npm:typescript@~5.9.3"); a project on 5.9 already, its own.
		const config = join(dir, "tsconfig.check.json");
		writeFileSync(config, tsconfig(entry));
		const sdkTsc = ["node_modules/typescript-sdk/bin/tsc", "../../node_modules/typescript-sdk/bin/tsc"].find(existsSync);
		try {
			await (sdkTsc ? runQuietly("node", [sdkTsc, "-p", config]) : runQuietly("tsc", ["-p", config]));
		} finally {
			rmSync(config, { force: true });
		}
		return;
	}
	if (existsSync(join(dir, "__init__.py")) && existsSync(join(dir, "client.py"))) return python(dir, group);
	if (existsSync(join(dir, "esm", "index.mjs")) && existsSync(join(dir, "esm", "index.d.mts"))) {
		console.log(`${dir}: compiled (esm/index.mjs and its types), as a release ships it`);
		return;
	}
	const cargo = join(dir, "Cargo.toml");
	if (existsSync(cargo)) {
		const bin = /\[\[bin\]\][^[]*?name\s*=\s*"([^"]+)"/.exec(readFileSync(cargo, "utf8"))?.[1];
		// A debug build: it proves the CLI compiles and links, in a tenth of a release build's time
		// (shipping one is a release's job). Fern writes the CLI's folder anew each time, its build
		// folder with it: Cargo builds into charter's cache instead (CARGO_TARGET_DIR), so the crates it
		// depends on are built once, and its own fingerprints say what to build again. A fresh machine,
		// as on GitHub, builds everything.
		await runQuietly("mise", ["exec", `rust@${rustVersion}`, "--", "cargo", "build", ...(bin ? ["--bin", bin] : []), "--no-default-features", "--features", "rustls"], { CARGO_TARGET_DIR: cacheFolder("cargo") }, dir);
		const binary = join(cacheFolder("cargo"), "debug", `${bin ?? "cli"}${process.platform === "win32" ? ".exe" : ""}`);
		if (existsSync(binary)) made(binary, "the CLI, a debug build");
		return;
	}
	console.log(`${dir}: generated; charter-ts proves Go, TypeScript, Python and the Rust CLI, not this language`);
}

/** prove generates every group the project declares and proves each, then, in a project that
 * commits its Go SDK (charter.config.ts: go.sdk), that sdk/go is what the specs generate now. */
export async function prove(only: string[], committedGoSdk: boolean): Promise<void> {
	const wanted = only.length > 0 ? only : groups();
	const start = performance.now();
	// Every client at once: Fern generates each in a Docker container of its own, and each is proven in
	// its language as soon as it is generated (the Rust CLI's build, the long one, starts early). What
	// they run keeps its output, shown only if it fails, so their lines do not mix; each says its times.
	console.log(`== generating and proving ${wanted.join(", ")}, all at once`);
	const results = await Promise.allSettled(wanted.map(async group => {
		const generated = await generateQuietly(group);
		console.log(`client ${group}: generated in ${seconds(generated)}`);
		const t0 = performance.now();
		await proveGroup(group);
		const proven = performance.now() - t0;
		console.log(`client ${group}: proven in ${seconds(proven)}`);
		return `${group} ${seconds(generated + proven)} (${seconds(generated)} + ${seconds(proven)})`;
	}));
	const failed = results.flatMap((result, i) => (result.status === "rejected" ? [`${wanted[i]}: ${(result.reason as Error).message}`] : []));
	if (failed.length > 0) throw new Error(`${failed.length} of the SDKs failed:\n${failed.join("\n")}`);
	const times = results.map(result => (result as PromiseFulfilledResult<string>).value);
	if (committedGoSdk && existsSync(join("sdk", "go")) && only.length === 0) {
		const t0 = performance.now();
		const { sdkPublish } = await import("./go.ts");
		await sdkPublish(true);
		times.push(`committed Go SDK ${seconds(performance.now() - t0)}`);
	}
	console.log(`sdk --prove took ${seconds(performance.now() - start)}, all at once: ${times.join(", ")} (generated + proven)`);
}

// What a project's fern/generators.yml must say that its spec already says: Fern sends only the
// credentials generators.yml restates (api.auth-schemes, and auth: any of them), so each security
// scheme of the spec is restated there, an API key with the spec's header, option name and variable.
// An OpenID Connect scheme is not: its token goes as the bearer token.
type Scheme = { type?: string; name?: string; "x-fern-header"?: { name?: string; env?: string } };

/** What fern/generators.yml lacks of the spec's schemes, one line each; none: nothing. */
export function unstatedSchemes(dir = "."): string[] {
	if (!existsSync(`${dir}/fern/openapi.json`) || !existsSync(`${dir}/fern/generators.yml`)) return [];
	const spec = JSON.parse(readFileSync(`${dir}/fern/openapi.json`, "utf8")) as { components?: { securitySchemes?: Record<string, Scheme> } };
	const yml = readFileSync(`${dir}/fern/generators.yml`, "utf8");
	const schemes = Object.entries(spec.components?.securitySchemes ?? {}).filter(([, scheme]) => scheme.type !== "openIdConnect");
	if (!schemes.length) return [];
	const missing: string[] = [];
	if (!/^ {2}auth:\n {4}any:\n/m.test(yml)) missing.push("api.auth is not `any:` of the schemes");
	for (const [name, scheme] of schemes) {
		const want = [`    ${name}:\n`, `      - ${name}\n`];
		if (scheme.type === "apiKey") want.push(`header: ${scheme.name}\n`, `name: ${scheme["x-fern-header"]?.name}\n`, `env: ${scheme["x-fern-header"]?.env}\n`);
		for (const line of want) if (!yml.includes(line)) missing.push(`the scheme ${name} has no ${JSON.stringify(line.trim())}`);
	}
	return missing;
}
