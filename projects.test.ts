// The projects of this repo are kept the same where they can be (docs/self/standard.md): the same
// npm scripts, in the same order, and one version of each package they share. A difference here is an
// accident, found before anyone reads the projects side by side.
//   node --test projects.test.ts   (npm run check runs it)
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { test } from "node:test";

type Package = { name: string; scripts?: Record<string, string>; dependencies?: Record<string, string>; devDependencies?: Record<string, string> };
const read = (path: string) => JSON.parse(readFileSync(path, "utf8")) as Package;
const root = read("package.json") as Package & { workspaces: string[] };
const projects = root.workspaces.filter(folder => existsSync(`${folder}/charter.config.ts`));

test("every project has the same scripts, Go or TypeScript: dev, check, spec, sdk, deploy, live-test", () => {
	const want = Object.fromEntries(["dev", "check", "spec", "sdk", "deploy", "live-test"].map(name => [name, `charter-ts ${name}`]));
	for (const folder of projects) assert.deepEqual(Object.entries(read(`${folder}/package.json`).scripts ?? {}), Object.entries(want), folder);
});

test("every script is the charter-ts command of its own name, so there is one way to run each (npm run <name>)", () => {
	// npm's own (prepare) and this repo's release are not charter-ts's.
	const scripts = [root, ...projects.map(folder => read(`${folder}/package.json`))].flatMap(pkg => Object.entries(pkg.scripts ?? {})).filter(([name]) => name !== "prepare" && name !== "release");
	assert.deepEqual(scripts.filter(([name, command]) => command !== `charter-ts ${name}`), []);
});

test("every charter-ts command is run by a script or a job of ci.config.ts (new and version by hand), and none by an old name", async () => {
	const { renamed, commands } = await import("./charter-ts/src/commands.ts");
	const { default: ci } = await import("./ci.config.ts");
	const scripts = [root, ...projects.map(folder => read(`${folder}/package.json`))].flatMap(pkg => Object.values(pkg.scripts ?? {}));
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
