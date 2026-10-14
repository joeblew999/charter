// What charter-ts's steps are made of, for a script of a project's own too: running a program, a dev
// server started and stopped around a test, the local database migrated, the Worker deployed with its
// secrets, and an SDK generated when the specs are newer, or copied into the Worker.
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { cpSync, existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { Config } from "./config.ts";

/** A program npm installed (cf, fern, tsc) runs from node_modules/.bin; any other from the path. */
function bin(program: string): string {
	const local = join("node_modules", ".bin", program);
	return existsSync(local) ? local : program;
}

/** run runs a program (in cwd, by default here) with the terminal's output, and throws if it fails. */
export function run(program: string, args: string[], env: Record<string, string> = {}, cwd?: string): void {
	console.log(`$ ${program} ${args.join(" ")}${cwd ? `   (in ${cwd})` : ""}`);
	const result = spawnSync(bin(program), args, { stdio: "inherit", cwd, env: { ...process.env, ...env }, shell: process.platform === "win32" });
	if (result.status !== 0) throw new Error(`${program} ${args.join(" ")} failed (${result.status ?? result.signal ?? result.error})`);
}

/** output runs a program (in cwd, by default here) and returns what it printed, and throws if it fails. */
export function output(program: string, args: string[], cwd?: string): string {
	const result = spawnSync(bin(program), args, { encoding: "utf8", cwd, shell: process.platform === "win32" });
	if (result.status !== 0) throw new Error(`${program} ${args.join(" ")} failed (${result.status ?? result.signal ?? result.error}): ${result.stderr}`);
	return result.stdout;
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
	const child = spawn(bin(server.program), server.args, { stdio: "inherit", env: { ...process.env, ...server.env }, detached: process.platform !== "win32", shell: process.platform === "win32" });
	try {
		await answers(server.ready, child);
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
 * project has migrations. `mode` deploys another of the config's modes (cf deploy --mode). */
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
	const database = `${project.worker}-db`;
	const found = (JSON.parse(output("cf", ["d1", "list", "--name", database, "--per-page", "100"])) as { name: string; uuid: string }[]).find(d => d.name === database);
	if (!found) throw new Error(`no D1 database ${database} after the deploy`);
	run("cf", ["d1", "migrations", "apply", found.uuid, "--dir", "migrations"]);
}

/** sdk generates a group of fern/generators.yml with Fern (Docker) into sdk/out/<group>. With
 * ifStale, only when there is no SDK yet or something in fern/ (the specs, Fern's settings) is
 * newer than it: the specs come from the contract, so a contract change reaches the SDK. */
export function sdk(group: string, options: { ifStale?: boolean } = {}): void {
	const made = join("sdk", "out", group);
	if (options.ifStale && existsSync(made)) {
		const at = statSync(made).mtimeMs;
		if (readdirSync("fern").every(file => statSync(join("fern", file)).mtimeMs <= at)) return;
	}
	run("fern", ["generate", "--local", "--group", group, "--force", "--log-level", "warn"]);
}

/** syncSdk copies the TypeScript SDK (typescript-dist) into folder, for a Worker that runs it, generating
 * it first if the specs are newer. */
export function syncSdk(folder: string): void {
	sdk("typescript-dist", { ifStale: true });
	rmSync(folder, { recursive: true, force: true });
	cpSync(join("sdk", "out", "typescript-dist", "esm"), folder, { recursive: true });
	console.log(`SDK copied into ${folder}`);
}
