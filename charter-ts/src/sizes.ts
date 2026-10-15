// What a command made, and how big: every artifact says its size, gzipped (what Cloudflare counts), how
// that changed since it was last made here, and the limit it is held to; check ends with them all.
import { existsSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { gzipSync } from "node:zlib";
import type { Config } from "./config.ts";
import { cacheFolder, output } from "./run.ts";

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

/** A file's size and its gzipped size (what Cloudflare counts); a folder's, the sum of its files', less
 * source maps (*.map), which a Worker does not run and its size limit does not count. */
function sizesOf(path: string): { raw: number; gzipped: number } {
	if (!statSync(path).isDirectory()) {
		const content = readFileSync(path);
		return { raw: content.length, gzipped: gzipSync(content).length };
	}
	return readdirSync(path, { recursive: true, withFileTypes: true })
		.filter(entry => entry.isFile() && !entry.name.endsWith(".map"))
		.map(entry => sizesOf(join(entry.parentPath, entry.name)))
		.reduce((sum, size) => ({ raw: sum.raw + size.raw, gzipped: sum.gzipped + size.gzipped }), { raw: 0, gzipped: 0 });
}

/**
 * made says what a command made, as every artifact charter-ts makes says it, so what grows shows where it
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

