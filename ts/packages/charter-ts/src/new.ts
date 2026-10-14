// charter-ts new: a new project, a copy of a tested example (start: one route; notes: the notes API,
// with auth, streams and rate limits) under its own name. The package carries the examples in
// templates/ (written when it is packed); in the charter repo they are read from ts/examples/.
import { cpSync, existsSync, mkdirSync, readdirSync, readFileSync, renameSync, statSync, writeFileSync } from "node:fs";
import { basename, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parseArgs } from "node:util";

export const templates = ["start", "notes"] as const;
export type Template = (typeof templates)[number];

/** What a copy leaves out: what npm, cf, Fern and the build write, and local secrets. */
const left = new Set(["node_modules", ".wrangler", ".cloudflare", "dist", "package-lock.json", ".DS_Store"]);
const leftPaths = new Set(["sdk/out", "sdk/.work", "src/client"]);
const leftFile = (name: string) => name.startsWith(".dev.vars") || name.endsWith(".log") || name.endsWith(".tsbuildinfo");

/** copyProject copies a project's own files from one folder to another. npm drops a package's
 * .gitignore files, so a template holds it as gitignore; `gitignore` says which name to write. */
export function copyProject(from: string, to: string, gitignore: ".gitignore" | "gitignore"): void {
	cpSync(from, to, {
		recursive: true,
		filter: source => {
			const path = relative(from, source).replaceAll("\\", "/");
			const name = basename(source);
			return path === "" || !(left.has(name) || leftPaths.has(path) || (statSync(source).isFile() && leftFile(name)));
		},
	});
	for (const name of [".gitignore", "gitignore"]) {
		if (name !== gitignore && existsSync(join(to, name))) renameSync(join(to, name), join(to, gitignore));
	}
}

/** The package's own folder: dist/.. when installed, src/.. when run from the repo. */
const packageDir = fileURLToPath(new URL("..", import.meta.url));

function templateDir(template: Template): string {
	const packed = join(packageDir, "templates", template);
	if (existsSync(packed)) return packed;
	const repo = join(packageDir, "..", "..", "examples", template);
	if (existsSync(repo)) return repo;
	throw new Error(`no template ${template} in ${packed}`);
}

/** The package's version, and the URL a project installs it from: the file on its GitHub Release. */
export function version(): string {
	return (JSON.parse(readFileSync(join(packageDir, "package.json"), "utf8")) as { version: string }).version;
}
export const releaseURL = (v: string) => `https://github.com/joeblew999/charter/releases/download/ts-v${v}/charter-ts-${v}.tgz`;

const text = /\.(ts|mts|js|mjs|json|ya?ml|toml|md|sql|html|css|txt)$|^gitignore$|^\.gitignore$/;

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

// The project's checks on GitHub: the same npm run check as on your machine.
const workflow = `# The project's checks on every push and pull request: npm run check, the same as on your machine.
name: check
on:
  push:
  pull_request:
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-node@v5
        with:
          node-version: 26
      - run: npm ci
      - run: npm run check
`;

export function newProject(args: string[]): void {
	const { values, positionals } = parseArgs({
		args,
		allowPositionals: true,
		options: { from: { type: "string", default: "start" }, subdomain: { type: "string" }, into: { type: "string" } },
	});
	const [name] = positionals;
	if (!name || !/^[a-z][a-z0-9-]*$/.test(name)) throw new Error("new needs a name of lowercase letters, digits and -, which is the Worker's name too: charter-ts new my-api");
	const from = values.from as Template;
	if (!templates.includes(from)) throw new Error(`--from is one of ${templates.join(", ")}`);
	const into = resolve(values.into ?? name);
	if (existsSync(into) && readdirSync(into).length > 0) throw new Error(`${into} is not empty`);
	mkdirSync(into, { recursive: true });
	copyProject(templateDir(from), into, ".gitignore");

	const old = `charter-${from}-ts`;
	const subdomain = values.subdomain ?? "your-subdomain";
	rewrite(into, (content, file) => {
		let next = content
			.replaceAll(old, name)
			.replaceAll(screaming(old), screaming(name))
			.replaceAll(".gedw99.workers.dev", `.${subdomain}.workers.dev`)
			.replace(/github\.com\/joeblew999\/charter\/[^\s"]+\/sdk\/go/g, `${name}/sdk/go`);
		if (basename(file) === "package.json" && file === join(into, "package.json")) {
			const pkg = JSON.parse(next) as { dependencies?: Record<string, string> };
			if (pkg.dependencies?.["charter-ts"]) pkg.dependencies["charter-ts"] = releaseURL(version());
			next = JSON.stringify(pkg, null, "\t") + "\n";
		}
		return next;
	});
	mkdirSync(join(into, ".github", "workflows"), { recursive: true });
	writeFileSync(join(into, ".github", "workflows", "check.yml"), workflow);

	console.log(`made ${into}, from the ${from} example, as the Worker ${name}`);
	console.log(`next:
  cd ${relative(process.cwd(), into) || "."}
  npm install          # the packages, charter-ts ${version()} among them
  npm run check        # typecheck, the specs, the tests under workerd: no Cloudflare account
  npm run dev          # the API on charter.config.ts's port
${values.subdomain ? "" : `Your workers.dev subdomain goes in charter.config.ts's url (cf deploy prints it; your-subdomain for now), then npm run spec.\n`}`);
}
