// Every charter-ts command: how it is called, where it works, what it does, when a developer runs it,
// and the code that runs it (each loads its modules only when it runs). A project's commands work in a
// folder with a charter.config.ts; the others in any repo.
import { createHash } from "node:crypto";
import { existsSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { deploy, migrateLocal } from "./cloudflare.ts";
import type { Config, Extras, Tests } from "./config.ts";
import { cacheFolder, freePort, run, seconds, untilStopped, withServer } from "./run.ts";
import { prove, sdk, syncSdk, unstatedSchemes } from "./sdk.ts";
import { bundle, madeThisRun, megabytes } from "./sizes.ts";

export interface Command {
	/** How it is called, with what it takes. */
	usage: string;
	/** Where it works: in a project (a folder with a charter.config.ts), in any repo, or both. */
	where: "project" | "repo" | "both";
	/** What it does, in one line. */
	about: string;
	/** When a developer runs it, for docs/charter/README.md; none for one a repo has no script of. */
	when?: string;
	run: (args: string[]) => void | Promise<void>;
}

async function load(): Promise<Config> {
	const file = resolve("charter.config.ts");
	if (!existsSync(file)) throw new Error(`no charter.config.ts in ${process.cwd()}: run it in a project's folder (charter-ts new <name> makes one)`);
	return ((await import(pathToFileURL(file).href)) as { default: Config }).default;
}

const go = () => import("./go.ts");

/** buildWorker readies the project's Worker before workerd runs it or a deploy ships it: a Go one built
 * with TinyGo, and the SDK a Worker runs itself (sdkInWorker) copied in. A TypeScript one needs no build. */
export async function buildWorker(config: Config, fresh = false): Promise<void> {
	if (config.go) (await go()).goBuild(config.go, config.bundleLimit, fresh);
	if (config.sdkInWorker) syncSdk(config.sdkInWorker);
}

/** testServer starts the API (program args) on a free port, migrates its local database, and runs the
 * standard test and then the project's tests against it. */
async function testServer(config: Config, tests: Tests, program: string, args: string[]): Promise<void> {
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

/** The commands, in the order a developer meets them. Each that a repo runs is the npm script of its name
 * (npm run <name>), so there is one way to run it; a project's are the same in Go and in TypeScript. */
export const commands: Record<string, Command> = {
	issues: {
		usage: "issues [bug|feature|upstream|plan]",
		where: "repo",
		about: "REMOTE, read-only: the open issues, newest first; with a kind (bug, feature, upstream, plan), the body of a new one and the gh command that files it",
		when: "First, whenever you start; with a kind, to file a bug or a plan",
		run: async args => (await import("./issues.ts")).issues(args),
	},
	dev: {
		usage: "dev [--native]",
		where: "project",
		about: "the API on your machine, as on Cloudflare (workerd), its local database migrated, until Ctrl-C; --native, in Go: go run .",
		when: "To try the API on your machine",
		run: async args => {
			const config = await load();
			const env = { ...config.localSecrets, PORT: String(config.port) };
			if (args.includes("--native")) {
				if (!config.go) throw new Error("dev --native is for a Go API: a TypeScript one runs only under workerd (npm run dev)");
				run("go", ["run", "."], env);
				return;
			}
			await buildWorker(config);
			await untilStopped({ program: "cf", args: ["dev"], env, ready: `http://localhost:${config.port}/api/openapi.json` }, async () => {
				if (existsSync("migrations")) await migrateLocal(config, config.port);
				console.log(`ready: http://localhost:${config.port}/`);
			});
		},
	},
	spec: { usage: "spec", where: "project", about: "write both specs (fern/openapi.json, fern/asyncapi.json) from the contract", when: "After every change to the contract", run: () => specs(false) },
	check: {
		usage: "check [<folder>] [--fresh]",
		where: "both",
		about: "every check that needs no Cloudflare account: in a project, the types, the specs against the contract, the tests, the API run and tested, its size; in a Go module, gofmt, go vet and go test; at a repo's top, the jobs ci.config.ts lists in check (gh signed in, for the repo job)",
		when: "Before every commit",
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
		where: "repo",
		about: "what a program can see wrong in docs/ (front matter, links and anchors, pages nothing links to, scripts and paths that don't exist, pinned release versions); --review has Claude read it for the rest",
		when: "After changing a page in docs/",
		run: async args => {
			if (args.includes("--review")) return (await import("./docs.ts")).docsReview(args.filter(arg => arg !== "--review"));
			(await import("./docs.ts")).docsLint();
		},
	},
	sdk: {
		usage: "sdk [<group>] [--prove [<group>...]] [--publish]",
		where: "project",
		about: "an SDK with Fern (Docker), by default the TypeScript one, into sdk/out; --prove: every client, proven in its language; --publish, in Go: the Go SDK into sdk/go",
		when: "When a caller needs an SDK; --prove after changing fern/generators.yml; --publish after a contract change, where the repo commits its Go SDK",
		run: async args => {
			const config = await load();
			const groups = args.filter(arg => !arg.startsWith("--"));
			if (args.includes("--publish")) {
				if (!config.go?.sdk) throw new Error("sdk --publish needs go.sdk in charter.config.ts: a Go project that commits its Go SDK");
				return await (await go()).sdkPublish(false);
			}
			if (args.includes("--prove")) return prove(groups, config.go?.sdk === true);
			sdk(groups[0] ?? "typescript-dist");
		},
	},
	deploy: {
		usage: "deploy",
		where: "project",
		about: "REMOTE: the Worker on Cloudflare with its secrets (fnox exec -- npm run deploy), then its migrations",
		when: "To put the API on Cloudflare",
		run: async () => {
			const config = await load();
			await buildWorker(config);
			deploy(config);
			for (const mode of config.deployModes ?? []) deploy(config, mode);
		},
	},
	"live-test": {
		usage: "live-test",
		where: "project",
		about: "REMOTE: the standard test and the project's tests against the deployed Worker",
		when: "After every deploy",
		run: async () => {
			const config = await load();
			if (!config.live) throw new Error("charter.config.ts has no live tests");
			await (await import("./standard.ts")).standard(config.url);
			await config.live({ url: config.url, node: (file, ...args) => run("node", [file, ...args]) });
		},
	},
	ci: {
		usage: "ci [<job> [<project>]] [--remote]",
		where: "repo",
		about: "a job of ci.config.ts here, as GitHub runs it; with no job, every job GitHub runs on a change; --remote starts the job on GitHub instead",
		when: "To run a job as GitHub does, here; --remote to start it there",
		run: async args => {
			const ci = await import("./ci.ts");
			const rest = args.filter(arg => arg !== "--remote");
			return args.includes("--remote") ? ci.ciRemote(rest) : ci.ci(rest);
		},
	},
	upstream: {
		usage: "upstream",
		where: "repo",
		about: "REMOTE, read-only: the upstream issues the code works around (its `Upstream:` tags), and their state",
		when: "Now and then: a CLOSED one means its workaround can go",
		run: async () => (await import("./upstream.ts")).upstream(),
	},
	repo: {
		usage: "repo [--check]",
		where: "repo",
		about: "REMOTE: the repo on GitHub as package.json says: labels, description, homepage, topics, GitHub Pages; --check changes nothing and fails on drift",
		when: "After changing package.json's description, keywords or homepage",
		run: async args => (await import("./repo.ts")).repo(args),
	},
	adopt: {
		usage: "adopt [--check] [--description <text>]",
		where: "repo",
		about: "write charter's files into the repo (never one of the repo's own), the scripts it lacks, and a workflow per job of ci.config.ts; --check writes nothing and fails if one differs",
		when: "After installing another release of charter-ts, and after changing ci.config.ts",
		run: async args => (await import("./adopt.ts")).adopt(args),
	},
	new: {
		usage: "new <name> [--from start|notes] [--go] [--subdomain <workers.dev subdomain>] [--into <dir>]",
		where: "repo",
		about: "a new project, a copy of a tested example under your name, in TypeScript or (--go) in Go, in a repo of its own that adopt keeps",
		run: async args => (await import("./new.ts")).newProject(args),
	},
	twins: {
		usage: "twins <reference> <other>",
		where: "repo",
		about: "a TypeScript project and its Go twin under workerd, the same requests to both, failing if an answer differs",
		run: async args => (await import("./twins.ts")).twins(args),
	},
	version: { usage: "version", where: "repo", about: "the version of charter-ts this is", run: async () => console.log((await import("./package.ts")).version()) },
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
	const times: [string, number][] = [];
	const timed = async (name: string, part: () => unknown) => {
		const start = performance.now();
		await part();
		times.push([name, performance.now() - start]);
	};
	await timed("lint", async () => {
		if (config.go) (await go()).goLint(config.go);
		const unstated = unstatedSchemes();
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
	if (config.native) await timed("native", () => testServer(config, config.native!, "go", ["run", "."]));
	await timed(config.go ? "build + workerd" : "workerd", async () => {
		await buildWorker(config, fresh);
		await testServer(config, config.test, "cf", ["dev"]);
	});
	// The Worker was built for workerd: measured as a deploy would upload it.
	await timed("size", () => bundle(config));
	const sizes = madeThisRun.map(({ what, gzipped, limit }) => `${what} ${megabytes(gzipped)} gzipped${limit ? ` of ${megabytes(limit)}` : ""}`);
	console.log(`check took ${seconds(times.reduce((sum, [, ms]) => sum + ms, 0))}: ${times.map(([name, ms]) => `${name} ${seconds(ms)}`).join(", ")}${sizes.length ? `; made ${sizes.join(", ")}` : ""}`);
}

/** fernCheck is Fern's check of the specs and its settings, skipped when it passed before for the same
 * files in fern/ and the same Fern: most of its time is Fern starting up. */
async function fernCheck(fresh: boolean): Promise<void> {
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
		run("go", ["run", config.specs.go, ...(check ? ["-check"] : []), "fern/openapi.json", "fern/asyncapi.json", server]);
		return;
	}
	const { writeSpecs } = await import("./specs.ts");
	await writeSpecs(config.specs, { server, check });
}
