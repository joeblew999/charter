// The projects of this repo are kept the same where they can be (docs/reference/standard.md): the same
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

test("every project's steps are the same scripts, in the same order: Go's add start, and sdk:publish where it may commit a Go SDK (go.sdk)", () => {
	for (const folder of projects) {
		const go = folder.startsWith("go/");
		const steps = [...(go ? ["start"] : []), "dev", "check", "spec", "sdk", ...(/\bsdk: true\b/.test(readFileSync(`${folder}/charter.config.ts`, "utf8")) ? ["sdk:publish"] : []), "deploy", "live-test"];
		const want = Object.fromEntries(steps.map(step => [step, `charter-ts ${step === "start" ? "run" : step}`]));
		assert.deepEqual(Object.entries(read(`${folder}/package.json`).scripts ?? {}), Object.entries(want), folder);
	}
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
