#!/usr/bin/env node
// charter-ts: a TypeScript API on Cloudflare Workers, step by step, from its charter.config.ts. A
// project's npm scripts call it (npm run dev runs charter-ts dev), so every step is the same on every
// project, on every machine and on GitHub.
import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import type { Config } from "./config.ts";
import { newProject, version } from "./new.ts";
import { deploy, freePort, migrateLocal, run, sdk, syncSdk, withServer } from "./scripts.ts";
import { writeSpecs } from "./spec-files.ts";

async function load(): Promise<Config> {
	const file = resolve("charter.config.ts");
	if (!existsSync(file)) throw new Error(`no charter.config.ts in ${process.cwd()}: run it in a project's folder (charter-ts new <name> makes one)`);
	return ((await import(pathToFileURL(file).href)) as { default: Config }).default;
}

const specs = async (check: boolean) => {
	const config = await load();
	await writeSpecs(config.specs, { server: config.specServer ?? config.url, check });
};

/** The steps, in the order a project runs them. */
const steps: Record<string, { usage: string; about: string; run: (args: string[]) => void | Promise<void> }> = {
	new: {
		usage: "new <name> [--from start|notes] [--subdomain <workers.dev subdomain>] [--into <dir>]",
		about: "a new project: a copy of a tested example under your name, with its GitHub workflow",
		run: newProject,
	},
	dev: {
		usage: "dev",
		about: "the API under workerd (cf dev) on the config's port, with the throwaway secrets; the first time, in another shell: migrate:local",
		run: async () => {
			const config = await load();
			if (config.sdkInWorker) syncSdk(config.sdkInWorker);
			run("cf", ["dev"], { ...config.localSecrets, PORT: String(config.port) });
		},
	},
	"migrate:local": {
		usage: "migrate:local",
		about: "apply migrations/*.sql, each once, to the local database of a running dev",
		run: async () => {
			const config = await load();
			await migrateLocal(config, config.port);
		},
	},
	lint: {
		usage: "lint",
		about: "typecheck the Worker (tsconfig.json, with cf's types) and what runs in Node: the config and the tests (tsconfig.node.json)",
		run: async () => {
			const config = await load();
			if (config.sdkInWorker) syncSdk(config.sdkInWorker);
			run("cf", ["workers", "types"]);
			run("tsc", []);
			if (existsSync("tsconfig.node.json")) run("tsc", ["-p", "tsconfig.node.json"]);
		},
	},
	spec: { usage: "spec", about: "write both specs from the contract into fern/", run: () => specs(false) },
	"spec:check": { usage: "spec:check", about: "fail if a committed spec is stale against the contract", run: () => specs(true) },
	"test:workerd": {
		usage: "test:workerd",
		about: "the Worker under workerd (cf dev) on a free port, its local database migrated, and the config's tests against it",
		run: async () => {
			const config = await load();
			if (config.sdkInWorker) syncSdk(config.sdkInWorker);
			const port = await freePort(), extra = await freePort();
			const origin = `http://localhost:${port}`;
			const env = { ...config.localSecrets, PORT: String(port), ...config.test.env?.({ port, extra }) };
			await withServer({ program: "cf", args: ["dev"], env, ready: origin + (config.test.ready ?? "/api/hello") }, async () => {
				if (existsSync("migrations")) await migrateLocal(config, port);
				await config.test.run({ origin, port, extra, node: (file, ...args) => run("node", [file, ...args], env) });
			});
		},
	},
	sdk: {
		usage: "sdk [group]",
		about: "generate a group of fern/generators.yml with Fern (Docker) into sdk/out/<group>; by default the TypeScript SDK",
		run: args => sdk(args[0] ?? "typescript-dist"),
	},
	deploy: {
		usage: "deploy",
		about: "REMOTE: deploy the Worker with its secrets from the environment (fnox exec -- npm run deploy), then apply its migrations",
		run: async () => {
			const config = await load();
			if (config.sdkInWorker) syncSdk(config.sdkInWorker);
			deploy(config);
			for (const mode of config.deployModes ?? []) deploy(config, mode);
		},
	},
	"live-test": {
		usage: "live-test",
		about: "REMOTE: the config's live tests against the deployed Worker (with the secrets: fnox exec -- npm run live-test)",
		run: async () => {
			const config = await load();
			if (!config.live) throw new Error("charter.config.ts has no live tests");
			await config.live({ url: config.url, node: (file, ...args) => run("node", [file, ...args]) });
		},
	},
	version: { usage: "version", about: "the version of charter-ts this is", run: () => console.log(version()) },
};

const [name, ...args] = process.argv.slice(2);
const step = name ? steps[name] : undefined;
if (!step) {
	console.log(`charter-ts ${version()}: a TypeScript API on Cloudflare Workers, from its charter.config.ts\n`);
	for (const s of Object.values(steps)) console.log(`  charter-ts ${s.usage}\n      ${s.about}`);
	process.exit(name ? 1 : 0);
}
try {
	await step.run(args);
} catch (error) {
	console.error(`charter-ts ${name}: ${error instanceof Error ? error.message : String(error)}`);
	process.exit(1);
}
