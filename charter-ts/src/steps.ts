// Every charter-ts command: what it takes, what it does, and where it works. A command loads its module
// only when it runs. Those for any repo keep the repo (its kit, its docs, its issues, its settings on
// GitHub); the others work in a project, a folder with a charter.config.ts.
import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import type { Config } from "./config.ts";

export interface Step {
	usage: string;
	about: string;
	/** Works in any repo, not only in a project. */
	anyRepo?: boolean;
	run: (args: string[]) => void | Promise<void>;
}

async function load(): Promise<Config> {
	const file = resolve("charter.config.ts");
	if (!existsSync(file)) throw new Error(`no charter.config.ts in ${process.cwd()}: run it in a project's folder (charter-ts new <name> makes one)`);
	return ((await import(pathToFileURL(file).href)) as { default: Config }).default;
}

const scripts = () => import("./scripts.ts");

/** The commands, in the order a repo and then a project run them. */
export const steps: Record<string, Step> = {
	adopt: {
		usage: "adopt [--description <text>] [--check]",
		about: "write charter's kit into the repo you are in, charter's own included: docs/charter/, the docs site's files, the issue forms, the workflow, AGENTS.md, CLAUDE.md, renovate.json, never a file of the repo's own; and what it lacks of package.json (charter-ts, the scripts), docs/README.md and docs/rules.md. Run again, it brings the kit to this version; --check writes nothing and fails if the kit differs",
		anyRepo: true,
		run: async args => (await import("./adopt.ts")).adopt(args),
	},
	repo: {
		usage: "repo [--check]",
		about: "keep the repo on GitHub in shape from package.json: labels, description, homepage, topics (its keywords), GitHub Pages; --check changes nothing and fails on drift",
		anyRepo: true,
		run: async args => (await import("./repo.ts")).repo(args),
	},
	issues: {
		usage: "issues",
		about: "REMOTE, read-only: the open issues, newest first: what is reported and what is planned (a plan is an issue)",
		anyRepo: true,
		run: async () => (await scripts()).run("gh", ["issue", "list", "--state", "open", "--limit", "100"]),
	},
	issue: {
		usage: "issue <bug|feature|upstream|plan>",
		about: "print an issue body with the headings of that issue form, and the gh command that files it",
		anyRepo: true,
		run: async args => (await import("./issue.ts")).issue(args),
	},
	"docs:lint": {
		usage: "docs:lint",
		about: "check docs/ for what a program can check: front matter, links, the index, scripts, tasks and paths that don't exist",
		anyRepo: true,
		run: async () => (await import("./docs-lint.ts")).docsLint(),
	},
	"upstream:status": {
		usage: "upstream:status",
		about: "REMOTE, read-only: the upstream issues the code works around (every `Upstream:` tag) and their state: CLOSED means that workaround can go",
		anyRepo: true,
		run: async () => (await import("./upstream.ts")).upstreamStatus(),
	},
	"docs:review": {
		usage: "docs:review [--print]",
		about: "have Claude (the claude command) bring docs/ up to date and into line with docs/charter/writing.md; --print only shows the prompt",
		anyRepo: true,
		run: async args => (await import("./review.ts")).docsReview(args),
	},
	new: {
		usage: "new <name> [--from start|notes] [--subdomain <workers.dev subdomain>] [--into <dir>]",
		about: "a new project: a copy of a tested example under your name, with its GitHub workflow",
		anyRepo: true,
		run: async args => (await import("./new.ts")).newProject(args),
	},
	dev: {
		usage: "dev",
		about: "the API under workerd (cf dev) on the config's port, with the throwaway secrets; the first time, in another shell: migrate:local",
		run: async () => {
			const config = await load();
			const { run, syncSdk } = await scripts();
			if (config.sdkInWorker) syncSdk(config.sdkInWorker);
			run("cf", ["dev"], { ...config.localSecrets, PORT: String(config.port) });
		},
	},
	"migrate:local": {
		usage: "migrate:local",
		about: "apply migrations/*.sql, each once, to the local database of a running dev",
		run: async () => {
			const config = await load();
			await (await scripts()).migrateLocal(config, config.port);
		},
	},
	lint: {
		usage: "lint",
		about: "typecheck the Worker (tsconfig.json, with cf's types) and what runs in Node: the config and the tests (tsconfig.node.json)",
		run: async () => {
			const config = await load();
			const { run, syncSdk } = await scripts();
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
			const { freePort, migrateLocal, run, syncSdk, withServer } = await scripts();
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
		run: async args => (await scripts()).sdk(args[0] ?? "typescript-dist"),
	},
	deploy: {
		usage: "deploy",
		about: "REMOTE: deploy the Worker with its secrets from the environment (fnox exec -- npm run deploy), then apply its migrations",
		run: async () => {
			const config = await load();
			const { deploy, syncSdk } = await scripts();
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
			const { run } = await scripts();
			await config.live({ url: config.url, node: (file, ...args) => run("node", [file, ...args]) });
		},
	},
	version: { usage: "version", about: "the version of charter-ts this is", anyRepo: true, run: async () => console.log((await import("./package.ts")).version()) },
};

async function specs(check: boolean): Promise<void> {
	const config = await load();
	const { writeSpecs } = await import("./spec-files.ts");
	await writeSpecs(config.specs, { server: config.specServer ?? config.url, check });
}
