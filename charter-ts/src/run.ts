// Running programs and servers, as every charter-ts command does: a program from the folder's packages
// or the path, its output shown or kept, a server started, waited for and stopped around what uses it,
// and how long each took; and charter's cache, where a command keeps what it may reuse.
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { existsSync, mkdirSync } from "node:fs";
import { createServer } from "node:net";
import { homedir } from "node:os";
import { dirname, join, resolve } from "node:path";

/** bin is a program as the folder's packages install it (node_modules/.bin, in the folder or a workspace
 * above it), or else by its name, from PATH. */
export function bin(program: string, cwd = "."): string {
	for (let dir = resolve(cwd); ; dir = dirname(dir)) {
		const local = join(dir, "node_modules", ".bin", program);
		if (existsSync(local)) return local;
		if (dirname(dir) === dir) return program;
	}
}

/** seconds is a duration as charter-ts prints every one: 12.3 s. */
export const seconds = (ms: number) => `${(ms / 1000).toFixed(1)} s`;

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
	const result = spawnSync(bin(program, cwd), args, { stdio: "inherit", cwd, env: { ...process.env, ...env }, shell: process.platform === "win32" });
	const took = performance.now() - start;
	if (result.status !== 0) throw new Error(`${program} ${args.join(" ")} failed (${result.status ?? result.signal ?? result.error}) after ${seconds(took)}`);
	if (took >= worthSaying) console.log(`took ${seconds(took)}: ${shortly(program, args)}`);
}

/** output runs a program (in cwd, by default here) and returns what it printed, and throws if it fails.
 * A run of a second or more says how long it took, on stderr: what it printed is the caller's. */
export function output(program: string, args: string[], cwd?: string, env: Record<string, string> = {}): string {
	const start = performance.now();
	const result = spawnSync(bin(program, cwd), args, { encoding: "utf8", cwd, env: { ...process.env, ...env }, shell: process.platform === "win32" });
	const took = performance.now() - start;
	if (result.status !== 0) throw new Error(`${program} ${args.join(" ")} failed (${result.status ?? result.signal ?? result.error}) after ${seconds(took)}: ${result.stderr}`);
	if (took >= worthSaying) console.error(`took ${seconds(took)}: ${shortly(program, args)}`);
	return result.stdout;
}

/** cacheFolder is a folder in charter's own cache (the user's cache folder: ~/Library/Caches/charter on
 * a Mac, ~/.cache/charter on Linux), made if it is not there: what a command may reuse when it did the
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
	/** The folder it runs in; by default this one. */
	cwd?: string;
}

/** keepAwake keeps a Mac from sleeping while a server runs: an idle Mac sleeps after a minute, and a
 * server that cannot answer then fails the tests against it. It ends with the server. */
function keepAwake(child: ChildProcess): void {
	if (process.platform === "darwin" && child.pid) spawn("caffeinate", ["-i", "-w", String(child.pid)], { stdio: "ignore", detached: true }).unref();
}

/** withServer starts a server, waits until it answers (two minutes at most), runs body, and stops the
 * server and everything it started, whether body passed or failed. */
export async function withServer<T>(server: Server, body: () => Promise<T>): Promise<T> {
	console.log(`$ ${server.program} ${server.args.join(" ")}   (started${server.cwd ? ` in ${server.cwd}` : ""})`);
	const start = performance.now();
	const child = spawn(bin(server.program, server.cwd), server.args, { stdio: "inherit", cwd: server.cwd, env: { ...process.env, ...server.env }, detached: process.platform !== "win32", shell: process.platform === "win32" });
	keepAwake(child);
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
	const child = spawn(bin(server.program, server.cwd), server.args, { stdio: "inherit", cwd: server.cwd, env: { ...process.env, ...server.env }, shell: process.platform === "win32" });
	keepAwake(child);
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

/** runQuietly runs a program with its output kept, not shown: for what runs beside others. It
 * resolves with that output and how long it took, and rejects with them if the program fails. */
export function runQuietly(program: string, args: string[], env: Record<string, string> = {}, cwd?: string): Promise<{ output: string; ms: number }> {
	const start = performance.now();
	return new Promise((resolve, reject) => {
		const child = spawn(bin(program, cwd), args, { cwd, env: { ...process.env, ...env }, shell: process.platform === "win32" });
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

