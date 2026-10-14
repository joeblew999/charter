// npm run release -- X.Y.Z [--dry-run]: a release of charter-ts, cut from this machine, onto GitHub.
//   1. The version: charter-ts's package.json and the examples' dependency on it, committed.
//   2. npm run check: every local check (Docker), the same one GitHub runs.
//   3. The packages (npm pack) into dist/: charter-ts (the command, the kit, the templates) and
//      charter-lib (what a TypeScript Worker imports), at the same version.
//   4. The tag ts-vX.Y.Z, pushed with main, and the GitHub Release, marked latest, with the package
//      on it: charter-ts-X.Y.Z.tgz, which a repo installs.
// The version goes into every command README.md and docs/ show too: npx keeps what it fetched for a
// URL, so only a URL with the version in it gives the version it names.
// Safe to run again: a step already done is skipped. --dry-run does steps 2 and 3 only, and changes no file.
import { spawn } from "node:child_process";
import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { releaseURL } from "charter-ts/package";
import { output, run } from "charter-ts/scripts";

// The Mac stays awake while the release runs (its check is minutes), and no longer.
if (process.platform === "darwin") spawn("caffeinate", ["-i", "-w", String(process.pid)], { stdio: "ignore", detached: true }).unref();

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
if (git("status", "--porcelain") !== "") throw new Error("the repo has changes that are not committed: commit them first");

// 1. The version.
// Both packages are released together, at one version: the tool and the library.
const manifests = ["charter-ts/package.json", "ts/lib/package.json"];
const pkg = JSON.parse(readFileSync(manifests[0], "utf8")) as { version: string };
if (pkg.version !== version && !dryRun) {
	console.log(`charter-ts and charter-lib ${pkg.version} -> ${version}`);
	for (const manifest of manifests) writeFileSync(manifest, JSON.stringify({ ...JSON.parse(readFileSync(manifest, "utf8")), version }, null, "\t") + "\n");
	const projects = (JSON.parse(readFileSync("package.json", "utf8")) as { workspaces: string[] }).workspaces.filter(w => w !== "charter-ts" && w !== "ts/lib");
	for (const project of projects) {
		const file = `${project}/package.json`;
		const manifestOf = JSON.parse(readFileSync(file, "utf8")) as { dependencies?: Record<string, string>; devDependencies?: Record<string, string> };
		for (const deps of [manifestOf.dependencies, manifestOf.devDependencies]) {
			for (const name of ["charter-ts", "charter-lib"]) if (deps?.[name]) deps[name] = `^${version}`;
		}
		writeFileSync(file, JSON.stringify(manifestOf, null, "\t") + "\n");
	}
	// The commands the docs show, at this version.
	const pages = ["README.md", ...walk("docs").filter(page => page.endsWith(".md") && !page.startsWith("docs/charter/"))];
	const changed = pages.filter(page => {
		const text = readFileSync(page, "utf8");
		const next = text.replace(/releases\/download\/ts-v\d+\.\d+\.\d+\/charter-ts-\d+\.\d+\.\d+\.tgz/g, `releases/download/ts-v${version}/charter-ts-${version}.tgz`);
		if (next !== text) writeFileSync(page, next);
		return next !== text;
	});
	run("npm", ["install", "--no-fund", "--no-audit"]);
	if (!dryRun) {
		run("git", ["add", "--", ...manifests, ...projects.map(project => `${project}/package.json`), "package-lock.json", ...changed]);
		run("git", ["commit", "-q", "-m", `charter-ts ${version}`]);
	}
}

// 2. Every local check.
run("npm", ["run", "check"]);

// 3. The packages (a dry run packs the version there is).
const packed = dryRun ? pkg.version : version;
const tarballs = [`dist/charter-ts-${packed}.tgz`, `dist/charter-lib-${packed}.tgz`];
mkdirSync("dist", { recursive: true });
run("npm", ["pack", "-w", "charter-ts", "-w", "ts/lib", "--pack-destination", resolve("dist")]);
for (const tarball of tarballs) if (!existsSync(tarball)) throw new Error(`npm pack wrote no ${tarball}`);
if (dryRun) {
	console.log(`dry run: ${tarballs.join(" and ")} are what the release would carry; nothing tagged, pushed or published`);
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

Keep a repo the charter way:

    npx --package ${releaseURL(version)} charter-ts adopt

Or move a repo to this release:

    npm i -D ${releaseURL(version)} && npm run adopt

A TypeScript Worker's library, charter-lib, is on the same release:

    npm i ${releaseURL(version, "charter-lib")}

Docs: https://joeblew999.github.io/charter/guides/typescript.html`;
if (!exists) run("gh", ["release", "create", tag, ...tarballs, "--title", `charter-ts ${version}`, "--notes", notes, "--latest"]);
else run("gh", ["release", "upload", tag, ...tarballs, "--clobber"]);
console.log(`released: ${releaseURL(version)}`);

function walk(dir: string): string[] {
	return readdirSync(dir, { withFileTypes: true }).flatMap(entry => (entry.isDirectory() ? walk(`${dir}/${entry.name}`) : [`${dir}/${entry.name}`]));
}
