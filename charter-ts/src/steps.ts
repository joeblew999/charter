// Every charter-ts command: what it takes, what it does, and where it works. A command loads its module
// only when it runs. Those for any repo keep the repo (its kit, its docs, its issues, its settings on
// GitHub); the others work in a project, a folder with a charter.config.ts.
import { existsSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import type { Config, Tests } from "./config.ts";

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
const go = () => import("./go.ts");

/** In a Go project, the Worker is built with TinyGo before workerd runs it or a deploy ships it. */
async function built(config: Config, fresh = false): Promise<void> {
	if (config.go) (await go()).goBuild(config.go, fresh);
	if (config.sdkInWorker) (await scripts()).syncSdk(config.sdkInWorker);
}

/** A server on a free port, its local database migrated, and the tests against it. */
async function serve(config: Config, tests: Tests, program: string, args: string[]): Promise<void> {
	const { freePort, migrateLocal, run, withServer } = await scripts();
	const port = await freePort(), extra = await freePort();
	const origin = `http://localhost:${port}`;
	const env = { ...config.localSecrets, PORT: String(port), ...tests.env?.({ port, extra }) };
	await withServer({ program, args, env, ready: origin + (tests.ready ?? "/api/hello") }, async () => {
		if (program === "cf" && existsSync("migrations")) await migrateLocal(config, port);
		await tests.run({ origin, port, extra, node: (file, ...a) => run("node", [file, ...a], env) });
	});
}

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
	"docs:check": {
		usage: "docs:check",
		about: "fail if a file of charter's kit is not what this version writes, or docs/ fails the lint",
		anyRepo: true,
		run: async () => {
			await steps.adopt.run(["--check"]);
			await steps["docs:lint"].run([]);
		},
	},
	"repo:ci": {
		usage: "repo:ci",
		about: "REMOTE, read-only: what the charter workflow runs, here: docs:check, repo --check, upstream:status. A pass here is a pass on GitHub",
		anyRepo: true,
		run: async () => {
			await steps["docs:check"].run([]);
			await steps.repo.run(["--check"]);
			await steps["upstream:status"].run([]);
		},
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
	build: {
		usage: "build",
		about: "Go: build the Worker into build/ with TinyGo, tuned for Workers, with the Go library's Worker glue beside it; fails over the size Workers Free allows",
		run: async () => {
			const config = await load();
			if (!config.go) throw new Error("build is for a Go Worker: cf builds a TypeScript one itself");
			(await go()).goBuild(config.go);
		},
	},
	run: {
		usage: "run",
		about: "Go: the API natively (go run .) on the config's port: no Cloudflare, an in-memory store",
		run: async () => {
			const config = await load();
			(await scripts()).run("go", ["run", "."], { ...config.localSecrets, PORT: String(config.port) });
		},
	},
	dev: {
		usage: "dev",
		about: "the API under workerd (cf dev) on the config's port, with the throwaway secrets (a Go Worker built first), its local database migrated once it answers; until Ctrl-C",
		run: async () => {
			const config = await load();
			await built(config);
			const { migrateLocal, untilStopped } = await scripts();
			const env = { ...config.localSecrets, PORT: String(config.port) };
			await untilStopped({ program: "cf", args: ["dev"], env, ready: `http://localhost:${config.port}${config.test.ready ?? "/api/hello"}` }, async () => {
				if (existsSync("migrations")) await migrateLocal(config, config.port);
				console.log(`ready: http://localhost:${config.port}/`);
			});
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
		about: "Go: gofmt, go vet for the host and for Wasm, gsx's output and the Go SDK fresh. TypeScript: typecheck the Worker (tsconfig.json, with cf's types) and what runs in Node: the config and the tests (tsconfig.node.json)",
		run: async () => {
			const config = await load();
			const { run, syncSdk } = await scripts();
			if (config.go) (await go()).goLint(config.go);
			if (config.sdkInWorker) syncSdk(config.sdkInWorker);
			if (existsSync("tsconfig.json")) {
				run("cf", ["workers", "types"]);
				run("tsc", []);
			}
			if (existsSync("tsconfig.node.json")) run("tsc", ["-p", "tsconfig.node.json"]);
		},
	},
	check: {
		usage: "check [--fresh]",
		about: "every local check, in order, with no Cloudflare account, and how long each took. Go: lint, go test, the specs, Fern's check, the tests natively and under workerd. TypeScript: lint, the specs, Fern's check, test/*.test.ts, the tests under workerd. A TinyGo build and Fern's check done before for the same sources come from charter's cache; --fresh does them again",
		run: async args => {
			const config = await load();
			const { run } = await scripts();
			const fresh = args.includes("--fresh");
			// Each part timed, and the times said at the end: where a slow check spends them.
			const times: [string, number][] = [];
			const timed = async (name: string, part: () => unknown) => {
				const start = performance.now();
				await part();
				times.push([name, performance.now() - start]);
			};
			await timed("lint", () => steps.lint.run([]));
			if (config.go) await timed("go test", () => run("go", ["test", "./..."]));
			await timed("specs", () => specs(true));
			if (existsSync("fern")) await timed("fern check", () => fernCheck(fresh));
			const unit = existsSync("test") ? readdirSync("test").filter(name => name.endsWith(".test.ts")).sort() : [];
			if (unit.length > 0) await timed("unit tests", () => run("node", ["--test", ...unit.map(name => `test/${name}`)]));
			if (config.native) await timed("native", () => steps["test:native"].run([]));
			await timed(config.go ? "build + workerd" : "workerd", () => steps["test:workerd"].run(fresh ? ["--fresh"] : []));
			const seconds = (ms: number) => `${(ms / 1000).toFixed(1)} s`;
			console.log(`check took ${seconds(times.reduce((sum, [, ms]) => sum + ms, 0))}: ${times.map(([name, ms]) => `${name} ${seconds(ms)}`).join(", ")}`);
		},
	},
	spec: { usage: "spec", about: "write both specs from the contract into fern/", run: () => specs(false) },
	"spec:check": { usage: "spec:check", about: "fail if a committed spec is stale against the contract", run: () => specs(true) },
	"test:native": {
		usage: "test:native",
		about: "Go: the API natively (go run .) on a free port, and the config's native tests against it",
		run: async () => {
			const config = await load();
			if (!config.native) throw new Error("charter.config.ts has no native tests");
			await serve(config, config.native, "go", ["run", "."]);
		},
	},
	"test:workerd": {
		usage: "test:workerd",
		about: "the Worker under workerd (cf dev) on a free port (a Go Worker built first), its local database migrated, and the config's tests against it",
		run: async args => {
			const config = await load();
			await built(config, args.includes("--fresh"));
			await serve(config, config.test, "cf", ["dev"]);
		},
	},
	sdk: {
		usage: "sdk [group]",
		about: "generate a group of fern/generators.yml with Fern (Docker) into sdk/out/<group>; by default the TypeScript SDK",
		run: async args => (await scripts()).sdk(args[0] ?? "typescript-dist"),
	},
	"sdk:publish": {
		usage: "sdk:publish [--check]",
		about: "Go: generate the Go SDK (Docker), prove it (build, vet, tests against WireMock) and copy its sources into sdk/go, the committed module another repo can go get; --check fails if sdk/go is not what the specs generate now",
		run: async args => (await go()).sdkPublish(args.includes("--check")),
	},
	deploy: {
		usage: "deploy",
		about: "REMOTE: deploy the Worker (a Go Worker built first) with its secrets from the environment (fnox exec -- npm run deploy), then apply its migrations",
		run: async () => {
			const config = await load();
			await built(config);
			const { deploy } = await scripts();
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

/** fernCheck is Fern's check of the specs and its settings, skipped when it passed before for the same
 * files in fern/ and the same Fern: most of its time is Fern starting up. */
async function fernCheck(fresh: boolean): Promise<void> {
	const { cacheFolder, run } = await scripts();
	const { createHash } = await import("node:crypto");
	const { createRequire } = await import("node:module");
	const { readFileSync, statSync, writeFileSync } = await import("node:fs");
	const { join } = await import("node:path");
	const hash = createHash("sha256");
	const fern = createRequire(join(process.cwd(), "package.json")).resolve("fern-api/package.json");
	hash.update(`fern-api ${(JSON.parse(readFileSync(fern, "utf8")) as { version: string }).version}\n`);
	const walk = (dir: string): string[] => readdirSync(dir).sort().flatMap(name => (statSync(join(dir, name)).isDirectory() ? walk(join(dir, name)) : [join(dir, name)]));
	for (const file of walk("fern")) {
		hash.update(`${file}\n`);
		hash.update(readFileSync(file));
	}
	const stamp = join(cacheFolder("fern-check"), hash.digest("hex").slice(0, 24));
	if (!fresh && existsSync(stamp)) {
		console.log("fern check: passed before for these specs and settings (charter's cache); --fresh runs it again");
		return;
	}
	run("fern", ["check", "--warnings"]);
	writeFileSync(stamp, "");
}

async function specs(check: boolean): Promise<void> {
	const config = await load();
	const server = config.specServer ?? config.url;
	if ("go" in config.specs) {
		(await scripts()).run("go", ["run", config.specs.go, ...(check ? ["-check"] : []), "fern/openapi.json", "fern/asyncapi.json", server]);
		return;
	}
	const { writeSpecs } = await import("./spec-files.ts");
	await writeSpecs(config.specs, { server, check });
}
