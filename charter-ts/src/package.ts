// charter-ts itself: where its files are, its version, and where a repo installs it from.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

/** The package's folder: dist/.. when installed, src/.. in the charter repo. */
export const packageDir = fileURLToPath(new URL("..", import.meta.url));

/** The version of charter-ts this is. */
export function version(): string {
	return (JSON.parse(readFileSync(join(packageDir, "package.json"), "utf8")) as { version: string }).version;
}

/** A package of a release on GitHub, by version: what a repo's package.json installs. Each release
 * carries both packages at its version: charter-ts (the tool) and charter-lib (what a Worker imports). */
export const releaseURL = (v: string, name: "charter-ts" | "charter-lib" = "charter-ts") => `https://github.com/joeblew999/charter/releases/download/ts-v${v}/${name}-${v}.tgz`;
