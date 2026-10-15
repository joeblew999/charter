// sdk --prove: a project's clients, every group of fern/generators.yml, generated with Fern (Docker) and proven in
// its own language. Go: build, vet, and Fern's tests against WireMock. TypeScript: typechecked with
// the TypeScript its SDKs are written for. Python: mypy and its own tests. The CLI (Rust): built. A
// language this does not know is generated and named, not proven. Fern generates in Docker; the
// toolchains a proof needs are mise's, at the versions pinned here, in mise's own folder.
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { cacheFolder, generateQuietly, made, output, runQuietly, seconds } from "./scripts.ts";

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
	const site = join("sdk", "out", `.${group}-import`);
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
const tsconfig = (entry: string) => JSON.stringify({
	"//": "Written by charter-ts sdk --prove: how a generated TypeScript SDK is typechecked (bundler mode, default libs).",
	compilerOptions: { target: "es2022", module: "preserve", moduleResolution: "bundler", allowImportingTsExtensions: true, strict: true, noEmit: true, skipLibCheck: true },
	include: [entry],
}, null, 2);

/** check proves a generated group: in its language, by what is in sdk/out/<group>. What it runs keeps
 * its output, shown only if it fails: the groups are proven side by side. */
export async function check(group: string): Promise<void> {
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

/** clients generates every group the project declares and proves each, then, in a project that
 * commits its Go SDK (charter.config.ts: go.sdk), that sdk/go is what the specs generate now. */
export async function clients(only: string[], committedGoSdk: boolean): Promise<void> {
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
		await check(group);
		const proven = performance.now() - t0;
		console.log(`client ${group}: proven in ${seconds(proven)}`);
		return `${group} ${seconds(generated + proven)} (${seconds(generated)} + ${seconds(proven)})`;
	}));
	const failed = results.flatMap((result, i) => (result.status === "rejected" ? [`${wanted[i]}: ${(result.reason as Error).message}`] : []));
	if (failed.length > 0) throw new Error(`${failed.length} of the clients failed:\n${failed.join("\n")}`);
	const times = results.map(result => (result as PromiseFulfilledResult<string>).value);
	if (committedGoSdk && existsSync(join("sdk", "go")) && only.length === 0) {
		const t0 = performance.now();
		const { sdkPublish } = await import("./go.ts");
		await sdkPublish(true);
		times.push(`committed Go SDK ${seconds(performance.now() - t0)}`);
	}
	console.log(`clients took ${seconds(performance.now() - start)}, all at once: ${times.join(", ")} (generated + proven)`);
}
