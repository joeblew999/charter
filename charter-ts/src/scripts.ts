// What charter-ts's steps are made of, for a script of a project's own too: running a program, a dev
// server started and stopped around a test, the local database migrated, the Worker deployed with its
// secrets, and an SDK generated when the specs are newer, or copied into the Worker.
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { cpSync, existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { createServer } from "node:net";
import { homedir, tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { gzipSync } from "node:zlib";
import type { Config } from "./config.ts";

/** A program npm installed (cf, fern, tsc) runs from node_modules/.bin; any other from the path. */
function bin(program: string): string {
	const local = join("node_modules", ".bin", program);
	return existsSync(local) ? local : program;
}

/** seconds is a duration as charter-ts prints every one: 12.3 s. */
export const seconds = (ms: number) => `${(ms / 1000).toFixed(1)} s`;

/** megabytes is a size as charter-ts prints every one: 1.20 MB, or 6.5 kB under a tenth of one. */
export const megabytes = (bytes: number) => (bytes < 100_000 ? `${(bytes / 1e3).toFixed(1)} kB` : `${(bytes / 1e6).toFixed(2)} MB`);

/** What this run made, in the order it made it: check ends its summary with it. */
export const madeThisRun: { what: string; gzipped: number; limit?: number }[] = [];

/** The sizes made before, by path: so a size says how it changed since. */
const sizesFile = () => join(cacheFolder(), "sizes.json");
function lastSizes(): Record<string, number> {
	try {
		return JSON.parse(readFileSync(sizesFile(), "utf8")) as Record<string, number>;
	} catch {
		return {};
	}
}

/** A file's size and its gzipped size (what Cloudflare counts); a folder's, the sum of its files'. */
function sizesOf(path: string): { raw: number; gzipped: number } {
	if (!statSync(path).isDirectory()) {
		const content = readFileSync(path);
		return { raw: content.length, gzipped: gzipSync(content).length };
	}
	return readdirSync(path, { recursive: true, withFileTypes: true })
		.filter(entry => entry.isFile())
		.map(entry => sizesOf(join(entry.parentPath, entry.name)))
		.reduce((sum, size) => ({ raw: sum.raw + size.raw, gzipped: sum.gzipped + size.gzipped }), { raw: 0, gzipped: 0 });
}

/**
 * made says what a step made, as every artifact charter-ts makes says it, so what grows shows where it
 * is made: its size, gzipped (what Cloudflare counts), how that changed since it was last made here,
 * and the limit it is held to, if any. A folder is the sum of its files. It returns the gzipped size.
 */
export function made(path: string, what: string, limit?: number): number {
	const { raw, gzipped } = sizesOf(path);
	const sizes = lastSizes(), key = resolve(path), before = sizes[key];
	const change = before === undefined ? "" : before === gzipped ? "; the same as last time" : `; ${gzipped > before ? "+" : "-"}${megabytes(Math.abs(gzipped - before))} gzipped since last time`;
	console.log(`made ${path} (${what}): ${megabytes(raw)}, ${megabytes(gzipped)} gzipped${limit ? ` (limit ${megabytes(limit)}${change})` : change ? ` (${change.slice(2)})` : ""}`);
	sizes[key] = gzipped;
	try {
		writeFileSync(sizesFile(), JSON.stringify(sizes, null, "\t"));
	} catch {
		// a size that is not remembered only loses its comparison next time
	}
	madeThisRun.push({ what, gzipped, limit });
	return gzipped;
}

/** The most a Worker's bundle may be, gzipped, unless charter.config.ts says otherwise: Workers Free's. */
export const freeBundleLimit = 3_000_000;

/** bundle builds the Worker as a deploy would upload it (cf build), says how big it is against its limit
 * (charter.config.ts: bundleLimit, Workers Free's 3 MB by default), and fails over it. */
export function bundle(project: Pick<Config, "bundleLimit">): void {
	output("cf", ["build"]);
	const workers = join(".cloudflare", "output", "v0", "workers");
	const limit = project.bundleLimit ?? freeBundleLimit;
	for (const name of existsSync(workers) ? readdirSync(workers) : []) {
		const folder = join(workers, name, "bundle");
		if (!existsSync(folder)) continue;
		if (made(folder, "the Worker's bundle", limit) > limit) {
			throw new Error(`the Worker's bundle is over its limit of ${megabytes(limit)} gzipped (charter.config.ts: bundleLimit; Workers Paid allows 10 MB)`);
		}
	}
}

/** The shortest run worth a line of its own: anything this long or longer says how long it took. */
const worthSaying = 1000;

/** A command line, short enough for a line that says how long it took. */
const shortly = (program: string, args: string[]) => {
	const line = `${program} ${args.join(" ")}`;
	return line.length > 100 ? `${line.slice(0, 97)}...` : line;
};

/** run runs a program (in cwd, by default here) with the terminal's output, and throws if it fails.
 * A run of a second or more says how long it took, so what is slow shows wherever it runs. */
export function run(program: string, args: string[], env: Record<string, string> = {}, cwd?: string): void {
	console.log(`$ ${program} ${args.join(" ")}${cwd ? `   (in ${cwd})` : ""}`);
	const start = performance.now();
	const result = spawnSync(bin(program), args, { stdio: "inherit", cwd, env: { ...process.env, ...env }, shell: process.platform === "win32" });
	const took = performance.now() - start;
	if (result.status !== 0) throw new Error(`${program} ${args.join(" ")} failed (${result.status ?? result.signal ?? result.error}) after ${seconds(took)}`);
	if (took >= worthSaying) console.log(`took ${seconds(took)}: ${shortly(program, args)}`);
}

/** output runs a program (in cwd, by default here) and returns what it printed, and throws if it fails.
 * A run of a second or more says how long it took, on stderr: what it printed is the caller's. */
export function output(program: string, args: string[], cwd?: string, env: Record<string, string> = {}): string {
	const start = performance.now();
	const result = spawnSync(bin(program), args, { encoding: "utf8", cwd, env: { ...process.env, ...env }, shell: process.platform === "win32" });
	const took = performance.now() - start;
	if (result.status !== 0) throw new Error(`${program} ${args.join(" ")} failed (${result.status ?? result.signal ?? result.error}) after ${seconds(took)}: ${result.stderr}`);
	if (took >= worthSaying) console.error(`took ${seconds(took)}: ${shortly(program, args)}`);
	return result.stdout;
}

/** cacheFolder is a folder in charter's own cache (the user's cache folder: ~/Library/Caches/charter on
 * a Mac, ~/.cache/charter on Linux), made if it is not there: what a step may reuse when it did the
 * same work before. A fresh machine, as on GitHub, has none, so it does everything. */
export function cacheFolder(...parts: string[]): string {
	const base =
		process.platform === "darwin" ? join(homedir(), "Library", "Caches")
		: process.platform === "win32" ? (process.env.LOCALAPPDATA ?? join(homedir(), "AppData", "Local"))
		: (process.env.XDG_CACHE_HOME ?? join(homedir(), ".cache"));
	const dir = join(base, "charter", ...parts);
	mkdirSync(dir, { recursive: true });
	return dir;
}

/** freePort is a port nothing listens on. */
export function freePort(): Promise<number> {
	return new Promise((resolve, reject) => {
		const server = createServer();
		server.on("error", reject);
		server.listen(0, () => {
			const address = server.address();
			server.close(() => (typeof address === "object" && address ? resolve(address.port) : reject(new Error("no free port"))));
		});
	});
}

/** A server withServer starts. */
export interface Server {
	program: string;
	args: string[];
	env: Record<string, string>;
	/** A URL that answers once the server is up. */
	ready: string;
}

/** withServer starts a server, waits until it answers (two minutes at most), runs body, and stops the
 * server and everything it started, whether body passed or failed. */
export async function withServer<T>(server: Server, body: () => Promise<T>): Promise<T> {
	console.log(`$ ${server.program} ${server.args.join(" ")}   (started)`);
	const start = performance.now();
	const child = spawn(bin(server.program), server.args, { stdio: "inherit", env: { ...process.env, ...server.env }, detached: process.platform !== "win32", shell: process.platform === "win32" });
	try {
		await answers(server.ready, child);
		console.log(`took ${seconds(performance.now() - start)}: ${server.program} answering at ${server.ready}`);
		return await body();
	} finally {
		stop(child);
	}
}

/** untilStopped starts a server, does `ready` once it answers, and runs until the server stops (Ctrl-C). */
export async function untilStopped(server: Server, ready: () => Promise<void>): Promise<void> {
	console.log(`$ ${server.program} ${server.args.join(" ")}`);
	const child = spawn(bin(server.program), server.args, { stdio: "inherit", env: { ...process.env, ...server.env }, shell: process.platform === "win32" });
	const exited = new Promise<number | null>(resolve => child.on("exit", code => resolve(code)));
	await answers(server.ready, child);
	await ready();
	const code = await exited;
	if (code) throw new Error(`${server.program} stopped (${code})`);
}

async function answers(url: string, child: ChildProcess): Promise<void> {
	const until = Date.now() + 120_000;
	while (Date.now() < until) {
		if (child.exitCode !== null) throw new Error(`the server stopped (${child.exitCode}) before ${url} answered`);
		try {
			if ((await fetch(url)).ok) return;
		} catch {
			// not up yet
		}
		await new Promise(resolve => setTimeout(resolve, 500));
	}
	throw new Error(`${url} did not answer in two minutes`);
}

function stop(child: ChildProcess): void {
	if (child.pid === undefined) return;
	try {
		if (process.platform === "win32") spawnSync("taskkill", ["/pid", String(child.pid), "/T", "/F"]);
		else process.kill(-child.pid, "SIGTERM");
	} catch {
		// it has stopped already
	}
}

/** migrateLocal applies migrations/*.sql, each once, to the local D1 database of the dev server on
 * port, through cf dev's local explorer. */
export async function migrateLocal(project: Pick<Config, "worker">, port: number): Promise<void> {
	const endpoint = `http://localhost:${port}/cdn-cgi/local/explorer/api/d1/database/DB-${project.worker}/raw`;
	const sql = async (statement: string): Promise<unknown> => {
		const reply = await fetch(endpoint, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ sql: statement }) });
		const body = (await reply.json()) as { success: boolean; errors: unknown[]; result?: { results?: { rows?: unknown[][] } }[] };
		if (!body.success) throw new Error(`local D1 refused (HTTP ${reply.status} ${JSON.stringify(body.errors)}): ${statement}`);
		return body.result?.at(-1)?.results?.rows?.[0]?.[0];
	};
	await sql("CREATE TABLE IF NOT EXISTS _local_migrations (name TEXT PRIMARY KEY, applied_at TEXT DEFAULT CURRENT_TIMESTAMP)");
	for (const name of readdirSync("migrations").filter(file => file.endsWith(".sql")).sort()) {
		if (Number(await sql(`SELECT count(*) FROM _local_migrations WHERE name = '${name}'`)) > 0) continue;
		await sql(readFileSync(join("migrations", name), "utf8"));
		await sql(`INSERT INTO _local_migrations (name) VALUES ('${name}')`);
		console.log("applied", name);
	}
}

/** deploy deploys the Worker with its secrets from the environment (never printed: they reach cf in a
 * file only this user can read, removed after), then applies migrations/ to its D1 database, if the
 * project has migrations. `mode` deploys another of the config's modes (cf deploy --mode): the Worker
 * <worker>-<mode>, as cloudflare.config.ts names it, with a database of its own. */
export function deploy(project: Pick<Config, "worker" | "secrets" | "optionalSecrets">, mode?: string): void {
	const secrets: Record<string, string> = {};
	for (const name of project.secrets ?? []) {
		const value = process.env[name];
		if (!value) throw new Error(`${name} is not set: the Worker needs it (fnox exec -- npm run deploy)`);
		secrets[name] = value;
	}
	for (const name of project.optionalSecrets ?? []) {
		const value = process.env[name];
		if (value) secrets[name] = value;
	}
	const args = ["deploy", ...(mode ? ["--mode", mode] : [])];
	const folder = mkdtempSync(join(tmpdir(), "charter-secrets-"));
	try {
		if (Object.keys(secrets).length > 0) {
			const file = join(folder, "secrets.json");
			writeFileSync(file, JSON.stringify(secrets), { mode: 0o600 });
			args.push("--secrets-file", file);
		}
		run("cf", args);
	} finally {
		rmSync(folder, { recursive: true, force: true });
	}
	if (!existsSync("migrations")) return;
	const database = `${project.worker}${mode ? `-${mode}` : ""}-db`;
	const found = (JSON.parse(output("cf", ["d1", "list", "--name", database, "--per-page", "100"])) as { name: string; uuid: string }[]).find(d => d.name === database);
	if (!found) throw new Error(`no D1 database ${database} after the deploy`);
	run("cf", ["d1", "migrations", "apply", found.uuid, "--dir", "migrations"]);
}

/** The hash of what an SDK is made from: every file in fern/ but the docs site's settings. */
export function specsHash(): string {
	const hash = createHash("sha256");
	for (const name of readdirSync("fern").sort()) {
		const file = join("fern", name);
		if (name === "docs.yml" || !statSync(file).isFile()) continue;
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
	madeFrom(group);
}

/** generateQuietly generates a group as sdk does, with Fern's output kept rather than shown, so that
 * several can run at once (each in a Docker container of its own) without their lines mixing. It
 * resolves with how long it took, and rejects with Fern's output if it fails. */
export async function generateQuietly(group: string): Promise<number> {
	const { ms } = await runQuietly("fern", fernGenerate(group));
	madeFrom(group);
	return ms;
}

const fernGenerate = (group: string) => ["generate", "--local", "--group", group, "--force", "--log-level", "warn"];
const stampOf = (group: string) => join("sdk", "out", `.${group}.made-from`);
const upToDate = (group: string) => existsSync(join("sdk", "out", group)) && existsSync(stampOf(group)) && readFileSync(stampOf(group), "utf8").trim() === specsHash();
function madeFrom(group: string): void {
	mkdirSync(join("sdk", "out"), { recursive: true });
	writeFileSync(stampOf(group), `${specsHash()}\n`);
}

/** runQuietly runs a program with its output kept, not shown: for what runs beside others. It
 * resolves with that output and how long it took, and rejects with them if the program fails. */
export function runQuietly(program: string, args: string[], env: Record<string, string> = {}, cwd?: string): Promise<{ output: string; ms: number }> {
	const start = performance.now();
	return new Promise((resolve, reject) => {
		const child = spawn(bin(program), args, { cwd, env: { ...process.env, ...env }, shell: process.platform === "win32" });
		let output = "";
		child.stdout.on("data", chunk => { output += chunk; });
		child.stderr.on("data", chunk => { output += chunk; });
		child.on("error", reject);
		child.on("close", code => {
			const ms = performance.now() - start;
			if (code === 0) resolve({ output, ms });
			else reject(new Error(`${program} ${args.join(" ")} failed (${code}) after ${seconds(ms)}:\n${output}`));
		});
	});
}

/** syncSdk copies the TypeScript SDK (typescript-dist) into folder, for a Worker that runs it, generating
 * it first if the specs are newer. */
export function syncSdk(folder: string): void {
	sdk("typescript-dist", { ifStale: true });
	rmSync(folder, { recursive: true, force: true });
	cpSync(join("sdk", "out", "typescript-dist", "esm"), folder, { recursive: true });
	console.log(`SDK copied into ${folder}`);
}
