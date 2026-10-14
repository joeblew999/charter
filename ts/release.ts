// npm run release -- X.Y.Z [--dry-run]: a release of charter-ts, cut from this machine, onto GitHub.
//   1. The version: charter-ts's package.json and the examples' dependency on it, committed.
//   2. npm run check: every local check (Docker), the same one GitHub runs.
//   3. The package (npm pack: the command, the library, the templates) into dist/.
//   4. The tag ts-vX.Y.Z, pushed with main, and the GitHub Release with the package on it, which a
//      project installs: npm i -D https://github.com/joeblew999/charter/releases/download/ts-vX.Y.Z/charter-ts-X.Y.Z.tgz
// Safe to run again: a step already done is skipped. --dry-run does steps 2 and 3 only, and changes no file.
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { releaseURL } from "charter-ts/new";
import { output, run } from "charter-ts/scripts";

process.on("uncaughtException", error => {
	console.error(`release: ${error.message}`);
	process.exit(1);
});

const [version] = process.argv.slice(2).filter(arg => !arg.startsWith("--"));
const dryRun = process.argv.includes("--dry-run");
if (!version || !/^\d+\.\d+\.\d+$/.test(version)) {
	console.error("usage: npm run release -- X.Y.Z [--dry-run]");
	process.exit(1);
}
const tag = `ts-v${version}`;
const git = (...args: string[]) => output("git", args).trim();

if (git("branch", "--show-current") !== "main") throw new Error("a release is cut from main");
if (git("status", "--porcelain", "--", ".") !== "") throw new Error("ts/ has changes that are not committed: commit them first");

// 1. The version.
const manifest = "packages/charter-ts/package.json";
const pkg = JSON.parse(readFileSync(manifest, "utf8")) as { version: string };
if (pkg.version !== version && !dryRun) {
	console.log(`charter-ts ${pkg.version} -> ${version}`);
	writeFileSync(manifest, JSON.stringify({ ...pkg, version }, null, "\t") + "\n");
	for (const example of ["start", "notes", "showcase"]) {
		const file = `examples/${example}/package.json`;
		const project = JSON.parse(readFileSync(file, "utf8")) as { dependencies: Record<string, string> };
		project.dependencies["charter-ts"] = `^${version}`;
		writeFileSync(file, JSON.stringify(project, null, "\t") + "\n");
	}
	run("npm", ["install", "--no-fund", "--no-audit"]);
	if (!dryRun) {
		run("git", ["add", "--", manifest, "examples/start/package.json", "examples/notes/package.json", "examples/showcase/package.json", "package-lock.json"]);
		run("git", ["commit", "-q", "-m", `charter-ts ${version}`]);
	}
}

// 2. Every local check.
run("npm", ["run", "check"]);

// 3. The package (a dry run packs the version there is).
const tarball = `dist/charter-ts-${dryRun ? pkg.version : version}.tgz`;
run("npm", ["pack", "-w", "charter-ts", "--pack-destination", "dist"]);
if (!existsSync(tarball)) throw new Error(`npm pack wrote no ${tarball}`);
if (dryRun) {
	console.log(`dry run: ${tarball} is what the release would carry; nothing tagged, pushed or published`);
	process.exit(0);
}

// 4. The tag and the GitHub Release.
if (git("tag", "-l", tag) === "") run("git", ["tag", tag]);
run("git", ["push", "-q", "origin", "main", tag]);
const exists = (() => {
	try {
		output("gh", ["release", "view", tag]);
		return true;
	} catch {
		return false;
	}
})();
const notes = `charter-ts ${version}: contract-first APIs on Cloudflare Workers in TypeScript.

Start a project:

    npx --package ${releaseURL(version)} charter-ts new my-api

Or add it to one:

    npm i -D ${releaseURL(version)}

Docs: https://joeblew999.github.io/charter/guides/typescript.html`;
if (!exists) run("gh", ["release", "create", tag, tarball, "--title", `charter-ts ${version}`, "--notes", notes, "--latest=false"]);
else run("gh", ["release", "upload", tag, tarball, "--clobber"]);
console.log(`released: ${releaseURL(version)}`);
