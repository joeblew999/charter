// The versions of the tools charter-ts runs, as mise.toml pins them: the one place they are said, so
// mise installs them, Renovate moves them, and every developer and CI get the same ones. A repo's own
// mise.toml (the nearest one above where charter-ts runs) wins; charter's (which the package carries)
// is what a repo without the pin gets.
import { existsSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { charterRepo, packageDir } from "./package.ts";

/** charter's own mise.toml: the repo's when charter-ts runs from it, else the copy the package carries. */
export const charterMise = () => join(charterRepo ?? join(packageDir, "charter-files"), "mise.toml");

/** The version a mise.toml's [tools] pins tool at, or undefined. A key is bare (go) or quoted
 * ("aqua:sqlc-dev/sqlc"); a value is "1.2.3" or { version = "1.2.3", ... }. */
export function pinIn(text: string, tool: string): string | undefined {
	let tools = false;
	for (const raw of text.split("\n")) {
		const line = raw.trim();
		if (line.startsWith("[")) {
			tools = line === "[tools]";
			continue;
		}
		if (!tools || line === "" || line.startsWith("#")) continue;
		const match = /^("([^"]+)"|([A-Za-z0-9_.:/@-]+))\s*=\s*(.+)$/.exec(line);
		if (!match || (match[2] ?? match[3]) !== tool) continue;
		const value = match[4]!;
		const version = /^"([^"]+)"/.exec(value)?.[1] ?? /version\s*=\s*"([^"]+)"/.exec(value)?.[1];
		if (version) return version;
	}
	return undefined;
}

/** The nearest mise.toml at or above dir. */
function nearestMise(dir: string): string | undefined {
	for (let at = resolve(dir); ; at = dirname(at)) {
		if (existsSync(join(at, "mise.toml"))) return join(at, "mise.toml");
		if (dirname(at) === at) return undefined;
	}
}

/** The version of tool to run: the nearest mise.toml's pin, else charter's. Throws when neither pins it,
 * so a tool charter-ts runs is never left to whatever version a machine has. */
export function pinned(tool: string, dir = process.cwd()): string {
	for (const file of [nearestMise(dir), charterMise()]) {
		if (!file || !existsSync(file)) continue;
		const version = pinIn(readFileSync(file, "utf8"), tool);
		if (version) return version;
	}
	throw new Error(`${tool} has no version in mise.toml's [tools] (here or charter's): pin it there`);
}

/** tool@version for mise exec, at the version pinned(tool) gives. */
export const miseTool = (tool: string, dir?: string) => `${tool}@${pinned(tool, dir)}`;
