// Every charter-ts command: what it takes, what it does, and where it works. A command loads its module
// only when it runs. A project's work in a folder with a charter.config.ts; those for any repo keep the
// repo (charter's files, its docs, its issues, its CI, its settings on GitHub).
import { existsSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import type { Config, Extras, Tests } from "./config.ts";

export interface Command {
	usage: string;
	/** What it does, in one line. */
	about: string;
	/** When a developer runs it: what docs/charter/README.md says (adopt writes it from the scripts). */
	when?: string;
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
	if (config.go) (await go()).goBuild(config.go, config.bundleLimit, fresh);
	if (config.sdkInWorker) (await scripts()).syncSdk(config.sdkInWorker);
}

/** A server on a free port, its local database migrated, and the tests against it. */
async function serve(config: Config, tests: Tests, program: string, args: string[]): Promise<void> {
	const { freePort, migrateLocal, run, withServer } = await scripts();
	// The server's port and three for what the tests serve, all different.
	const ports = new Set<number>();
	while (ports.size < 4) ports.add(await freePort());
	const [port, ...rest] = [...ports] as [number, number, number, number];
	const extras = rest as unknown as Extras;
	const origin = `http://localhost:${port}`;
	const env = { ...config.localSecrets, PORT: String(port), ...tests.env?.({ port, extras }) };
	const server = () => withServer({ program, args, env, ready: `${origin}/api/openapi.json` }, async () => {
		if (program === "cf" && existsSync("migrations")) await migrateLocal(config, port);
		// What every API must serve, first (the standard test, docs/self/standard.md); then the project's own tests.
		await (await import("./standard.ts")).standard(origin);
		await tests.run({ origin, port, extras, node: (file, ...a) => run("node", [file, ...a], env) });
	});
	// What the tests serve, up before the server and down after it: each wraps the next.
	const helpers = tests.serve?.({ port, extras }) ?? [];
	await helpers.reduceRight<() => Promise<void>>((inner, helper) => () => withServer({ ...helper, env }, inner), server)();
}

/** The commands, in the order a developer meets them. Each is the npm script of its name in every repo
 * that runs it (npm run <name>), so there is one way to run each; a project's are the same in Go and in
 * TypeScript. Those without anyRepo work in a project, a folder with a charter.config.ts. */
export const commands: Record<string, Command> = {
	issues: {
		usage: "issues [bug|feature|upstream|plan]",
		about: "REMOTE, read-only: the open issues, newest first; with a kind (bug, feature, upstream, plan), the body of a new one and the gh command that files it",
		when: "First, whenever you start: one labelled needs-triage comes before anything else. With a kind, to file one: a plan or a bug is an issue, never a page",
		anyRepo: true,
		run: async args => {
			if (args[0]) return (await import("./issue.ts")).issue(args);
			(await scripts()).run("gh", ["issue", "list", "--state", "open", "--limit", "100"]);
		},
	},
	dev: {
		usage: "dev [--native]",
		about: "the API on your machine, as on Cloudflare (workerd), its local database migrated, until Ctrl-C; --native, in Go: go run .",
		when: "To try the API on your machine",
		run: async args => {
			const config = await load();
			const { migrateLocal, run, untilStopped } = await scripts();
			const env = { ...config.localSecrets, PORT: String(config.port) };
			if (args.includes("--native")) {
				if (!config.go) throw new Error("dev --native is for a Go API: a TypeScript one runs only under workerd (npm run dev)");
				run("go", ["run", "."], env);
				return;
			}
			await built(config);
			await untilStopped({ program: "cf", args: ["dev"], env, ready: `http://localhost:${config.port}/api/openapi.json` }, async () => {
				if (existsSync("migrations")) await migrateLocal(config, config.port);
				console.log(`ready: http://localhost:${config.port}/`);
			});
		},
	},
	spec: { usage: "spec", about: "write both specs (fern/openapi.json, fern/asyncapi.json) from the contract", when: "After every change to the contract", run: () => specs(false) },
	check: {
		usage: "check [<folder>] [--fresh]",
		about: "every check that needs no Cloudflare account: in a project, the types, the specs, the tests, the API run and tested, its size; in a Go module, gofmt, go vet and go test; at a repo's top, the jobs ci.config.ts lists in check",
		when: "Before every commit",
		anyRepo: true,
		run: async args => {
			const folder = args.find(arg => !arg.startsWith("--"));
			if (folder) {
				if (existsSync(join(folder, "charter.config.ts"))) process.chdir(folder);
				else if (existsSync(join(folder, "go.mod"))) return (await go()).goCheck(folder);
				else throw new Error(`${folder} is neither a project (charter.config.ts) nor a Go module (go.mod)`);
			}
			if (!existsSync("charter.config.ts")) return (await import("./ci.ts")).ciCheck();
			await projectCheck(args.includes("--fresh"));
		},
	},
	docs: {
		usage: "docs [--review [--print]]",
		about: "check docs/ for what a program can (front matter, links, scripts and paths that don't exist); --review has Claude bring docs/ into line with docs/charter/writing.md",
		when: "After changing a page in docs/",
		anyRepo: true,
		run: async args => {
			if (args.includes("--review")) return (await import("./review.ts")).docsReview(args.filter(arg => arg !== "--review"));
			(await import("./docs-lint.ts")).docsLint();
		},
	},
	sdk: {
		usage: "sdk [<group>] [--prove [<group>...]] [--publish]",
		about: "an SDK with Fern (Docker), by default the TypeScript one, into sdk/out; --prove: every client, proven in its language; --publish, in Go: the Go SDK into sdk/go",
		when: "When a caller needs an SDK; --prove after changing fern/generators.yml; --publish after a contract change, where the repo commits its Go SDK",
		run: async args => {
			const config = await load();
			const groups = args.filter(arg => !arg.startsWith("--"));
			if (args.includes("--publish")) {
				if (!config.go?.sdk) throw new Error("sdk --publish needs go.sdk in charter.config.ts: a Go project that commits its Go SDK");
				return await (await go()).sdkPublish(false);
			}
			if (args.includes("--prove")) return (await import("./clients.ts")).clients(groups, config.go?.sdk === true);
			(await scripts()).sdk(groups[0] ?? "typescript-dist");
		},
	},
	deploy: {
		usage: "deploy",
		about: "REMOTE: the Worker on Cloudflare with its secrets (fnox exec -- npm run deploy), then its migrations",
		when: "To put the API on Cloudflare",
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
		about: "REMOTE: the standard test and the project's tests against the deployed Worker",
		when: "After every deploy",
		run: async () => {
			const config = await load();
			if (!config.live) throw new Error("charter.config.ts has no live tests");
			const { run } = await scripts();
			await (await import("./standard.ts")).standard(config.url);
			await config.live({ url: config.url, node: (file, ...args) => run("node", [file, ...args]) });
		},
	},
	ci: {
		usage: "ci [<job> [<project>]] [--remote]",
		about: "a job of ci.config.ts here, as GitHub runs it; with no job, every job GitHub runs on a change; --remote starts the job on GitHub instead",
		when: "To run a job as GitHub does, here; --remote to start it there",
		anyRepo: true,
		run: async args => {
			const ci = await import("./ci.ts");
			const rest = args.filter(arg => arg !== "--remote");
			return args.includes("--remote") ? ci.ciRemote(rest) : ci.ci(rest);
		},
	},
	upstream: {
		usage: "upstream",
		about: "REMOTE, read-only: the upstream issues the code works around (its `Upstream:` tags), and their state",
		when: "Now and then: a CLOSED one means its workaround can go",
		anyRepo: true,
		run: async () => (await import("./upstream.ts")).upstream(),
	},
	repo: {
		usage: "repo [--check]",
		about: "REMOTE: the repo on GitHub as package.json says: labels, description, homepage, topics, GitHub Pages; --check changes nothing and fails on drift",
		when: "After changing package.json's description, keywords or homepage",
		anyRepo: true,
		run: async args => (await import("./repo.ts")).repo(args),
	},
	adopt: {
		usage: "adopt [--check] [--description <text>]",
		about: "write charter's files into the repo (never one of the repo's own), the scripts it lacks, and a workflow per job of ci.config.ts; --check writes nothing and fails if one differs",
		when: "After installing another release of charter-ts, and after changing ci.config.ts",
		anyRepo: true,
		run: async args => (await import("./adopt.ts")).adopt(args),
	},
	new: {
		usage: "new <name> [--from start|notes] [--go] [--subdomain <workers.dev subdomain>] [--into <dir>]",
		about: "a new project, a copy of a tested example under your name, in TypeScript or (--go) in Go, in a repo of its own that adopt keeps",
		anyRepo: true,
		run: async args => (await import("./new.ts")).newProject(args),
	},
	twins: {
		usage: "twins <reference> <other>",
		about: "a TypeScript project and its Go twin under workerd, the same requests to both, failing if an answer differs",
		anyRepo: true,
		run: async args => (await import("./twins.ts")).twins(args),
	},
	version: { usage: "version", about: "the version of charter-ts this is", anyRepo: true, run: async () => console.log((await import("./package.ts")).version()) },
};

/** What a command was called before, and what it is now: a repo's old script says where it went. */
export const renamed: Record<string, string> = {
	run: "dev --native",
	build: "check (it builds the Worker)",
	bundle: "check (it measures the Worker)",
	lint: "check",
	"spec:check": "check",
	"test:native": "check",
	"test:workerd": "check",
	"migrate:local": "dev (it migrates the local database)",
	clients: "sdk --prove",
	"sdk:publish": "sdk --publish",
	"go:check": "check <folder>",
	"docs:lint": "docs",
	"docs:check": "adopt --check, then docs",
	"docs:review": "docs --review",
	"repo:ci": "ci repo",
	"upstream:status": "upstream",
	issue: "issues <kind>",
	"ci:write": "adopt (it writes the workflows)",
	"ci:remote": "ci <job> --remote",
};

/** A project's check: each part timed, and the times said at the end, with what it made. */
async function projectCheck(fresh: boolean): Promise<void> {
	const config = await load();
	const { bundle, madeThisRun, megabytes, run, seconds, syncSdk } = await scripts();
	const times: [string, number][] = [];
	const timed = async (name: string, part: () => unknown) => {
		const start = performance.now();
		await part();
		times.push([name, performance.now() - start]);
	};
	await timed("lint", async () => {
		if (config.go) (await go()).goLint(config.go);
		const unstated = (await import("./fern.ts")).unstatedSchemes();
		if (unstated.length) throw new Error(`fern/generators.yml does not restate the spec's security schemes, so the SDKs would not send them:\n  ${unstated.join("\n  ")}`);
		if (config.sdkInWorker) syncSdk(config.sdkInWorker);
		if (existsSync("tsconfig.json")) {
			run("cf", ["workers", "types"]);
			run("tsc", []);
		}
		if (existsSync("tsconfig.node.json")) run("tsc", ["-p", "tsconfig.node.json"]);
	});
	if (config.go) await timed("go test", () => run("go", ["test", "./..."]));
	await timed("specs", () => specs(true));
	if (existsSync("fern")) await timed("fern check", () => fernCheck(fresh));
	// Unit tests sit beside the code they test (api/*.test.ts), as Go's _test.go files do.
	const unit = existsSync("api") ? readdirSync("api").filter(name => name.endsWith(".test.ts")).sort() : [];
	if (unit.length > 0) await timed("unit tests", () => run("node", ["--test", ...unit.map(name => `api/${name}`)]));
	if (config.native) await timed("native", () => serve(config, config.native!, "go", ["run", "."]));
	await timed(config.go ? "build + workerd" : "workerd", async () => {
		await built(config, fresh);
		await serve(config, config.test, "cf", ["dev"]);
	});
	// The Worker was built for workerd: measured as a deploy would upload it.
	await timed("bundle", () => bundle(config));
	const sizes = madeThisRun.map(({ what, gzipped, limit }) => `${what} ${megabytes(gzipped)} gzipped${limit ? ` of ${megabytes(limit)}` : ""}`);
	console.log(`check took ${seconds(times.reduce((sum, [, ms]) => sum + ms, 0))}: ${times.map(([name, ms]) => `${name} ${seconds(ms)}`).join(", ")}${sizes.length ? `; made ${sizes.join(", ")}` : ""}`);
}

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
