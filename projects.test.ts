// The projects of this repo are kept the same where they can be (docs/self/standard.md): no npm scripts (a
// project's steps are charter's, from its charter.toml), one version of each package they share, and one
// TypeScript setup. A difference here is an accident, found before anyone reads the projects side by side.
//   node --test projects.test.ts   (npm run check runs it)
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { test } from "node:test";

type Package = { name: string; scripts?: Record<string, string>; dependencies?: Record<string, string>; devDependencies?: Record<string, string> };
const read = (path: string) => JSON.parse(readFileSync(path, "utf8")) as Package;
const root = read("package.json") as Package & { workspaces: string[] };
// A project is charter's (charter.toml, #121): its steps are the Go command's (go tool charter, mise run charter:*).
const projects = root.workspaces.filter(folder => existsSync(`${folder}/charter.toml`));

test("a project has no npm scripts and none of cf's or charter-ts's files: its steps are charter's, from its charter.toml", () => {
	assert.ok(projects.length >= 8, `the projects are charter.toml's: ${projects.join(", ")}`);
	for (const folder of projects) {
		assert.deepEqual(read(`${folder}/package.json`).scripts ?? {}, {}, folder);
		for (const file of ["charter.config.ts", "cloudflare.config.ts", "vite.config.ts"]) assert.ok(!existsSync(`${folder}/${file}`), `${folder}: ${file}`);
	}
});

test("every script is the charter-ts command of its own name, so there is one way to run each (npm run <name>)", () => {
	// npm's own (prepare) and this repo's release are not charter-ts's.
	const scripts = Object.entries(root.scripts ?? {}).filter(([name]) => name !== "prepare" && name !== "release");
	assert.deepEqual(scripts.filter(([name, command]) => command !== `charter-ts ${name}`), []);
});

test("every charter-ts command is run by a script or a job of ci.config.ts (new and version by hand), and none by an old name", async () => {
	const { renamed, commands } = await import("./charter-ts/src/commands.ts");
	const { default: ci } = await import("./ci.config.ts");
	const scripts = Object.values(root.scripts ?? {});
	const used = new Set([...scripts.filter(command => command.startsWith("charter-ts ")).map(command => command.split(" ")[1]), ...Object.values(ci.jobs).flatMap(job => job.run.filter(command => command[0] === "charter-ts").map(command => command[1])), "new", "version"]);
	assert.deepEqual(Object.keys(commands).filter(name => !used.has(name)), [], "commands nothing runs");
	assert.deepEqual([...used].filter(name => name && !commands[name!]), [], "run, but not a command");
	assert.deepEqual([...used].filter(name => name && renamed[name!]), [], "an old name");
});

test("a package two projects use is at one version in both", () => {
	const versions = new Map<string, Map<string, string[]>>();
	for (const folder of projects) {
		const pkg = read(`${folder}/package.json`);
		for (const [name, version] of Object.entries({ ...pkg.dependencies, ...pkg.devDependencies })) {
			const byVersion = versions.get(name) ?? new Map<string, string[]>();
			byVersion.set(version, [...(byVersion.get(version) ?? []), folder]);
			versions.set(name, byVersion);
		}
	}
	const differ = [...versions].filter(([, byVersion]) => byVersion.size > 1).map(([name, byVersion]) => `${name}: ${[...byVersion].map(([version, folders]) => `${version} in ${folders.join(", ")}`).join("; ")}`);
	assert.deepEqual(differ, []);
});

test("every project typechecks with the same TypeScript, and has the one Fern's TypeScript SDK builds with", () => {
	for (const folder of projects) {
		const dev = read(`${folder}/package.json`).devDependencies ?? {};
		assert.ok(dev.typescript && dev["typescript-sdk"], `${folder}: typescript and typescript-sdk`);
	}
});

test("every project's tsconfig.json (the Worker's) and tsconfig.node.json (its tests') have the same options: each its own, so the project stands alone", () => {
	for (const file of ["tsconfig.json", "tsconfig.node.json"]) {
		const options = projects.filter(folder => existsSync(`${folder}/${file}`)).map(folder => [folder, JSON.stringify((JSON.parse(readFileSync(`${folder}/${file}`, "utf8")) as { compilerOptions: unknown }).compilerOptions)] as const);
		assert.ok(options.length > 0, `no project has a ${file}`);
		const differ = options.filter(([, o]) => o !== options[0]![1]).map(([folder]) => folder);
		assert.deepEqual(differ, [], `${file}'s compilerOptions differ from ${options[0]![0]}'s`);
	}
});

test("every package is pinned by version, not by a tag that moves (beta, latest): a project made by new installs what was tested", () => {
	const tagged = [root, read("charter-ts/package.json"), ...projects.map(folder => read(`${folder}/package.json`))]
		.flatMap(pkg => Object.entries({ ...pkg.dependencies, ...pkg.devDependencies }).map(([name, version]) => `${pkg.name}: ${name}@${version}`))
		.filter(entry => /@(latest|beta|next|canary|alpha|\*)$/.test(entry));
	assert.deepEqual(tagged, []);
});
