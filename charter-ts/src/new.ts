// charter-ts new: a new project, a copy of one of charter's tested examples (start: one route; notes:
// the notes API, with sign-in, streams and rate limits), in TypeScript or, with --go, in Go, under its
// own name, in a repo of its own that adopt keeps. The examples are charter's repo's, as the release
// has them: charter-ts fetches the repo at its own tag (ts-v<version>) from GitHub; run from charter's
// own checkout, it copies them from there.
import { cpSync, existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { basename, join, relative, resolve } from "node:path";
import { parseArgs } from "node:util";
import { charterRepo, releaseURL, version } from "./package.ts";
import { output } from "./run.ts";

export const examples = ["start", "notes"] as const;
type Example = (typeof examples)[number];
/** A project's language: TypeScript (oRPC) or Go (Huma). */
type Language = "ts" | "go";

/** What a copy leaves out: what npm, cf, Fern and the build write, and local secrets. */
const left = new Set(["node_modules", ".wrangler", ".cloudflare", "dist", "build", "package-lock.json", ".DS_Store"]);
const leftPaths = new Set(["sdk/out"]);
const leftFile = (name: string) => name.startsWith(".dev.vars") || name.endsWith(".log") || name.endsWith(".tsbuildinfo");

/** copyProject copies a project's own files from one folder to another, less the paths in `also`. */
function copyProject(from: string, to: string, also: string[] = []): void {
	cpSync(from, to, {
		recursive: true,
		filter: source => {
			const path = relative(from, source).replaceAll("\\", "/");
			const name = basename(source);
			return path === "" || !(left.has(name) || leftPaths.has(path) || also.includes(path) || (statSync(source).isFile() && leftFile(name)));
		},
	});
}

/** copyExample copies an example of charter's repo (at repo) into `to`: a TypeScript one as it is; a Go
 * one with its TypeScript twin's tests, which in the repo it runs where they are, and the schemas they
 * check its answers against, so that it stands alone. Its committed Go SDK is the example's (a project
 * publishes its own), and so is the soak test of a redeploy (test/soak*), which charter runs by hand. */
function copyExample(repo: string, example: Example, language: Language, to: string): void {
	const twin = join(repo, "ts", `examples-${example}`);
	if (language === "ts") return copyProject(twin, to);
	copyProject(join(repo, "go", `examples-${example}`), to, ["sdk/go", "test/soak.ts", "test/soak-go"]);
	cpSync(join(twin, "test"), join(to, "test"), { recursive: true });
	cpSync(join(twin, "api", "schemas.ts"), join(to, "test", "schemas.ts"));
	rewrite(join(to, "test"), content => content.replaceAll('from "../api/schemas.ts"', 'from "./schemas.ts"'));
	rewrite(to, (content, file) => (basename(file) === "charter.config.ts" ? content.replaceAll(`../../ts/examples-${example}/test`, "test") : content));
	const tsconfig = join(to, "tsconfig.node.json");
	writeFileSync(tsconfig, JSON.stringify({ ...JSON.parse(readFileSync(tsconfig, "utf8")), include: ["charter.config.ts", "test"] }, null, "\t") + "\n");
}

/** The repo the examples are copied from: charter's own checkout, when charter-ts runs from it; else the
 * repo at this release's tag, from GitHub, or from the archive CHARTER_SOURCE names (charter's test of
 * new). It is fetched into a folder of its own, which done removes. */
async function examplesRepo(): Promise<{ root: string; done: () => void }> {
	if (charterRepo) return { root: charterRepo, done: () => {} };
	const scratch = mkdtempSync(join(tmpdir(), "charter-examples-"));
	const done = () => rmSync(scratch, { recursive: true, force: true });
	try {
		let archive = process.env.CHARTER_SOURCE;
		if (!archive) {
			const url = `https://github.com/joeblew999/charter/archive/refs/tags/ts-v${version()}.tar.gz`;
			const response = await fetch(url);
			if (!response.ok) throw new Error(`the examples of charter-ts ${version()} could not be fetched: ${url} answered ${response.status}`);
			archive = join(scratch, "charter.tar.gz");
			writeFileSync(archive, Buffer.from(await response.arrayBuffer()));
		}
		const into = join(scratch, "repo");
		mkdirSync(into);
		output("tar", ["-xzf", resolve(archive), "-C", into]);
		// GitHub's archive has the repo in one folder (charter-ts-v<version>); one of the files alone has none.
		const top = readdirSync(into);
		return { root: top.length === 1 && !existsSync(join(into, "ts")) ? join(into, top[0]!) : into, done };
	} catch (error) {
		done();
		throw error;
	}
}

const text = /\.(ts|mts|js|mjs|json|ya?ml|toml|md|sql|html|css|txt|go)$|^go\.mod$|^gitignore$|^\.gitignore$/;

function rewrite(dir: string, change: (content: string, file: string) => string): void {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const file = join(dir, entry.name);
		if (entry.isDirectory()) rewrite(file, change);
		else if (text.test(entry.name)) {
			const before = readFileSync(file, "utf8");
			const after = change(before, file);
			if (after !== before) writeFileSync(file, after);
		}
	}
}

const screaming = (name: string) => name.toUpperCase().replaceAll("-", "_");

export async function newProject(args: string[]): Promise<void> {
	const { values, positionals } = parseArgs({
		args,
		allowPositionals: true,
		options: { from: { type: "string", default: "start" }, go: { type: "boolean" }, subdomain: { type: "string" }, into: { type: "string" } },
	});
	const [name] = positionals;
	if (!name || !/^[a-z][a-z0-9-]*$/.test(name)) throw new Error("new needs a name of lowercase letters, digits and -, which is the Worker's name too: charter-ts new my-api");
	const from = values.from as Example;
	if (!examples.includes(from)) throw new Error(`--from is one of ${examples.join(", ")}`);
	const into = resolve(values.into ?? name);
	if (existsSync(into) && readdirSync(into).length > 0) throw new Error(`${into} is not empty`);
	mkdirSync(into, { recursive: true });
	const language: Language = values.go ? "go" : "ts";
	const repo = await examplesRepo();
	try {
		copyExample(repo.root, from, language, into);
	} finally {
		repo.done();
	}

	const old = `charter-${from}-${language}`;
	const subdomain = values.subdomain ?? "your-subdomain";
	rewrite(into, (content, file) => {
		let next = content
			.replaceAll(old, name)
			.replaceAll(screaming(old), screaming(name))
			.replaceAll(".gedw99.workers.dev", `.${subdomain}.workers.dev`)
			// In Go: the project's module is its name, and the Go library is the release's, not the repo's.
			.replaceAll(`github.com/joeblew999/charter/go/examples-${from}`, name)
			.replace(/github\.com\/joeblew999\/charter\/[^\s"]+\/sdk\/go/g, `${name}/sdk/go`);
		if (basename(file) === "go.mod") next = next.replace(/\nreplace github\.com\/joeblew999\/charter\/go\/lib => \S+\n/, "\n").replace(/(github\.com\/joeblew999\/charter\/go\/lib) v0\.0\.0/, `$1 v${version()}`);
		if (basename(file) === "package.json" && file === join(into, "package.json")) {
			const pkg = JSON.parse(next) as { dependencies?: Record<string, string>; devDependencies?: Record<string, string> };
			for (const deps of [pkg.dependencies, pkg.devDependencies]) {
				if (deps?.["charter-lib"]) deps["charter-lib"] = releaseURL(version(), "charter-lib");
				if (deps?.["charter-ts"]) deps["charter-ts"] = releaseURL(version());
			}
			next = JSON.stringify(pkg, null, "\t") + "\n";
		}
		return next;
	});
	console.log(`made ${into}, from the ${from} example in ${language === "go" ? "Go" : "TypeScript"}, as the Worker ${name}`);
	// Go: the Go library of the release, fetched (go.sum).
	let tidy = true;
	if (language === "go") {
		try {
			output("go", ["mod", "tidy"], into);
		} catch {
			tidy = false;
			console.log(`go mod tidy failed: the Go library v${version()} could not be fetched. Run it again in ${into} when you can reach GitHub.`);
		}
	}

	// A project is a repo of its own, kept as every repo is: adopt writes its CI (ci.config.ts, and its
	// workflows from it), its docs and charter's files.
	if (!existsSync(join(into, ".git"))) output("git", ["init", "-q"], into);
	const here = process.cwd();
	process.chdir(into);
	try {
		await (await import("./adopt.ts")).adopt([]);
	} finally {
		process.chdir(here);
	}
	console.log(`next:
  cd ${relative(process.cwd(), into) || "."}
${tidy ? "" : "  go mod tidy          # the Go library\n"}  npm install          # the packages, charter-ts ${version()} among them
  npm run check        # every check that needs no Cloudflare account
  npm run dev          # the API on charter.config.ts's port
${values.subdomain ? "" : `Your workers.dev subdomain goes in charter.config.ts's url (cf deploy prints it; your-subdomain for now), then npm run spec.\n`}`);
}
