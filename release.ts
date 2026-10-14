// npm run release -- X.Y.Z [--dry-run]: a release of charter-ts, cut from this machine, onto GitHub.
//   1. The version: charter-ts's package.json and the examples' dependency on it, committed.
//   2. npm run check: every local check (Docker), the same one GitHub runs.
//   3. The package (npm pack: the command, the library, the templates) into dist/.
//   4. The tag ts-vX.Y.Z, pushed with main, and the GitHub Release, marked latest, with the package
//      on it: charter-ts-X.Y.Z.tgz, which a repo installs.
// The version goes into every command README.md and docs/ show too: npx keeps what it fetched for a
// URL, so only a URL with the version in it gives the version it names.
// Safe to run again: a step already done is skipped. --dry-run does steps 2 and 3 only, and changes no file.
import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { releaseURL } from "charter-ts/package";
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
if (git("status", "--porcelain") !== "") throw new Error("the repo has changes that are not committed: commit them first");

// 1. The version.
const manifest = "charter-ts/package.json";
const pkg = JSON.parse(readFileSync(manifest, "utf8")) as { version: string };
if (pkg.version !== version && !dryRun) {
	console.log(`charter-ts ${pkg.version} -> ${version}`);
	writeFileSync(manifest, JSON.stringify({ ...pkg, version }, null, "\t") + "\n");
	const projects = (JSON.parse(readFileSync("package.json", "utf8")) as { workspaces: string[] }).workspaces.filter(w => w !== "charter-ts");
	for (const project of projects) {
		const file = `${project}/package.json`;
		const manifestOf = JSON.parse(readFileSync(file, "utf8")) as { dependencies?: Record<string, string>; devDependencies?: Record<string, string> };
		const deps = manifestOf.dependencies?.["charter-ts"] ? manifestOf.dependencies : (manifestOf.devDependencies ??= {});
		deps["charter-ts"] = `^${version}`;
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
		run("git", ["add", "--", manifest, ...projects.map(project => `${project}/package.json`), "package-lock.json", ...changed]);
		run("git", ["commit", "-q", "-m", `charter-ts ${version}`]);
	}
}

// 2. Every local check.
run("npm", ["run", "check"]);

// 3. The package (a dry run packs the version there is).
const tarball = `dist/charter-ts-${dryRun ? pkg.version : version}.tgz`;
mkdirSync("dist", { recursive: true });
run("npm", ["pack", "-w", "charter-ts", "--pack-destination", resolve("dist")]);
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

Keep a repo the charter way:

    npx --package ${releaseURL(version)} charter-ts adopt

Or move a repo to this release:

    npm i -D ${releaseURL(version)} && npm run adopt

Docs: https://joeblew999.github.io/charter/guides/typescript.html`;
if (!exists) run("gh", ["release", "create", tag, tarball, "--title", `charter-ts ${version}`, "--notes", notes, "--latest"]);
else run("gh", ["release", "upload", tag, tarball, "--clobber"]);
console.log(`released: ${releaseURL(version)}`);

function walk(dir: string): string[] {
	return readdirSync(dir, { withFileTypes: true }).flatMap(entry => (entry.isDirectory() ? walk(`${dir}/${entry.name}`) : [`${dir}/${entry.name}`]));
}
