// charter's files: what charter keeps the same in every repo, as charter's own repo has them (its pages
// in docs/charter/, which say what to do and when; the docs site's files GitHub Pages reads at the top of docs/, the issue forms, the
// labels and the job runner in .github/, AGENTS.md, CLAUDE.md, renovate.json). They are changed there,
// and only there: the package carries a copy (charter-files/, which npm pack takes from the repo). Charter writes
// a file only where it may: in its own places always, elsewhere where there is none, or the one there
// carries its mark. What a repo wrote itself is never written over.
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { basename, dirname, join, posix } from "node:path";
import { charterRepo, packageDir } from "./package.ts";
import { pinIn } from "./pins.ts";
import { isProject, readPackage, settings } from "./settings.ts";
import { charterJobs, workflow } from "./ci.ts";
import { type Command, commands } from "./commands.ts";

export const charterFolder = "charter";

/** charter's files, by path in a repo. */
export const charterFiles = [
	"AGENTS.md",
	"CLAUDE.md",
	"renovate.json",
	".github/charter-labels.tsv",
	".github/ISSUE_TEMPLATE/config.yml",
	".github/ISSUE_TEMPLATE/charter-bug.yml",
	".github/ISSUE_TEMPLATE/charter-feature.yml",
	".github/ISSUE_TEMPLATE/charter-plan.yml",
	".github/ISSUE_TEMPLATE/charter-upstream.yml",
	".github/workflows/charter-ci.yml",
	"docs/_config.yml",
	"docs/_sass/custom/custom.scss",
	"docs/llms.txt",
	"docs/charter/rules.md",
	"docs/charter/writing.md",
];

/** Where charter's file is: in charter's own repo, the file itself; in a package, its copy. */
export const charterFile = (rel: string) => join(charterRepo ?? join(packageDir, "charter-files"), rel);

const startMark = "Started by `charter-ts adopt` (github.com/joeblew999/charter): yours to edit.";

/** The tools a repo needs, as mise.toml pins them (mise installs them, here and on GitHub), at the
 * versions charter's own mise.toml pins: node for charter-ts; in a project, fnox for its secrets; in
 * a Go one, go, and the TinyGo, wasm-opt and sqlc its Worker and queries are built with. */
export function miseStarter(project: boolean, go: boolean): string {
	const charters = readFileSync(charterFile("mise.toml"), "utf8");
	const key = (tool: string) => (/^[A-Za-z0-9_-]+$/.test(tool) ? tool : JSON.stringify(tool));
	const pin = (tool: string) => {
		const version = pinIn(charters, tool);
		if (!version) throw new Error(`charter's mise.toml has no pin for ${tool}`);
		return `${key(tool)} = ${JSON.stringify(version)}`;
	};
	const tools = ["node", ...(project ? ["github:jdx/fnox"] : []), ...(go ? ["go", "tinygo", "aqua:WebAssembly/binaryen", "aqua:sqlc-dev/sqlc"] : [])];
	return `# ${startMark}\n# The tools, which mise installs here and on GitHub (charter-ci.yml): ${tools.join(", ")}.\n[tools]\n${tools.map(pin).join("\n")}\n`;
}

/** What adopt starts a repo with where it has none, then the repo's own: the tools it needs
 * (mise.toml), its line ends and generated files (.gitattributes), a project's CI (its check job), and
 * in docs/self/, the page that says what it is and its own rules. */
const starters = (project: boolean, go: boolean): Record<string, string> => ({
	"mise.toml": miseStarter(project, go),
	".gitattributes": `# ${startMark}
# Line ends are LF on every system: the checks compare generated files with the committed ones byte
# for byte, and gofmt wants LF. (Git on Windows would otherwise check files out with CRLF.)
* text=auto eol=lf

# Generated files: nobody edits these by hand. GitHub marks them "generated" and folds them in diffs.
fern/openapi.json                   linguist-generated=true
fern/asyncapi.json                  linguist-generated=true
sdk/go/**                           linguist-generated=true
*.x.go                              linguist-generated=true
package-lock.json                   linguist-generated=true
.github/workflows/self-*.yml        linguist-generated=true
.github/workflows/charter-repo.yml  linguist-generated=true
docs/charter/README.md              linguist-generated=true
`,
	...(project && {
		"ci.config.ts": `// ${startMark}
// This project's CI: its own jobs, and when GitHub runs them, typed. \`npm run adopt\` writes a workflow
// per job (.github/workflows/self-<job>.yml), each calling charter's job runner (charter-ci.yml);
// \`npm run ci -- <job>\` runs one here, the same way. charter's own job, repo, is charter's
// (charter-repo.yml). Add a job of your own: { about, on, run: [[program, ...args]] }.
import type { CI } from "charter-ts/ci";

export default {
	jobs: {
		check: {
			about: "Every check that needs no Cloudflare account: npm run check",
			on: ["push", "pull_request"],
			run: [["charter-ts", "check"]],
			timeoutMinutes: 30,
		},
	},
} satisfies CI;
`,
	}),
	"docs/self/README.md": `---
# ${startMark}
title: Start here
nav_order: 1
permalink: /
---

# Start here

This repo's own pages; \`AGENTS.md\` only points here. Say here what this repo is, and where its parts are. What to do, and when: [docs/charter/](../charter/README.md).

| Page | What it is |
|---|---|
| [rules.md](rules.md) | This repo's own rules |
`,
	"docs/self/rules.md": `---
# ${startMark}
title: Rules
nav_order: 2
---

# Rules for working in this repo

The rules every repo shares are in [Rules for every repo](../charter/rules.md). Add here what is this repo's own.
`,
});

/** A file's mark, in its first lines: Written by `charter-ts <command>` ... don't edit (charter's), or
 * Started by `charter-ts <command>` ... yours to edit (the repo's, written once). */
export function markOf(content: string): { charters: boolean; command: string } | undefined {
	const head = content.split("\n").slice(0, 12).join("\n");
	const m = /(Written|Started) by `charter-ts ([a-z-]+)`/.exec(head);
	if (m?.[1] === "Written" && head.includes("don't edit")) return { charters: true, command: m[2] };
	if (m?.[1] === "Started" && head.includes("yours to edit")) return { charters: false, command: m[2] };
	return undefined;
}

/** marked: the file is there and is charter's. */
export const marked = (path: string) => existsSync(path) && markOf(readFileSync(path, "utf8"))?.charters === true;

function walk(dir: string, prefix = ""): string[] {
	if (!existsSync(dir)) return [];
	return readdirSync(dir, { withFileTypes: true }).flatMap(entry =>
		entry.isDirectory() ? walk(join(dir, entry.name), `${prefix}${entry.name}/`) : [`${prefix}${entry.name}`],
	);
}

const cell = (text: string) => text.replaceAll("|", "\\|").replaceAll("\n", " ");

/** docs/charter/README.md: what to do, and when, written from the repo's scripts (each the charter-ts
 * command of its name) in the order a developer meets them, then the repo's own scripts. */
function whatToDoPage(root: string): string {
	const scripts = readPackage(root)?.scripts ?? {};
	const charters = Object.entries(commands).filter(([name]) => scripts[name] === `charter-ts ${name}`);
	const own = Object.entries(scripts).filter(([name, command]) => command !== `charter-ts ${name}` && !["prepare", "preinstall", "postinstall", "install"].includes(name));
	const row = (name: string, c: Command, where = "") => `| ${cell(c.when ?? "")} | \`npm run ${name}${where}\` | ${cell(c.about)} |`;
	const rows = charters.map(([name, c]) => row(name, c));
	// A repo whose workspaces are projects: their scripts too, run from here with -w.
	const projects = (readPackage(root)?.workspaces ?? []).filter(dir => isProject(join(root, dir)));
	const projectScripts = projects.length ? readPackage(join(root, projects[0]!))?.scripts ?? {} : {};
	const inProjects = Object.entries(commands).filter(([name]) => projectScripts[name] === `charter-ts ${name}` && scripts[name] === undefined);
	return `---
title: What to do
nav_order: 100
has_children: true
---

# What to do, and when

Written by \`charter-ts adopt\` from the repo's scripts (package.json): don't edit, change the scripts, then \`npm run adopt\`.

Each is \`npm run <name>\`; what it takes goes after \`--\` (\`npm run issues -- plan\`). [The rules](rules.md) are binding; [Writing docs](writing.md) says how a page is written; this repo's own pages are in [self/](../self/README.md).

| When | Run | What it does |
|---|---|---|
${rows.join("\n")}
${inProjects.length ? `\nIn a project (${projects.map(dir => `\`${dir}\``).join(", ")}), from its folder, or from here with \`-w <folder>\`:\n\n| When | Run | What it does |\n|---|---|---|\n${inProjects.map(([name, c]) => row(name, c)).join("\n")}\n` : ""}${own.length ? `\nThe repo's own: ${own.map(([name, command]) => `\`npm run ${name}\` (\`${command}\`)`).join(", ")}.\n` : ""}
Another release of charter-ts: \`npm i -D https://github.com/joeblew999/charter/releases/download/ts-vX.Y.Z/charter-ts-X.Y.Z.tgz\`, then \`npm run adopt\` (Renovate opens that pull request, on a repo with its app installed). Every command, with what it takes: \`npx charter-ts help\`.
`;
}

/** charter's files for the repo at root, by path in the repo: each as charter has it, less what the repo
 * does not take (renovate.json; with "workflow": false, the workflows and the CI starter); the starters;
 * docs/charter/README.md, written from the repo's scripts; and charter's job workflows (charter-<job>.yml). */
function filesFor(root: string): Map<string, string> {
	const s = settings(root);
	// The workflows: charter's jobs, and the runner every workflow calls (unless "workflow": false).
	const workflows = s.workflow;
	const inDocs = (rel: string) => (rel.startsWith("docs/") ? posix.join(s.docs, rel.slice("docs/".length)) : rel);
	const files = new Map<string, string>();
	for (const rel of charterFiles) {
		if (rel === "renovate.json" && !s.renovate) continue;
		if (rel === ".github/workflows/charter-ci.yml" && !workflows) continue;
		files.set(inDocs(rel), readFileSync(charterFile(rel), "utf8"));
	}
	for (const [rel, content] of Object.entries(starters(isProject(root), existsSync(join(root, "go.mod"))))) {
		if (rel === "ci.config.ts" && !workflows) continue;
		files.set(inDocs(rel), content);
	}
	files.set(posix.join(s.docs, charterFolder, "README.md"), whatToDoPage(root));
	if (workflows) for (const [name, job] of Object.entries(charterJobs)) files.set(`.github/workflows/charter-${name}.yml`, workflow(root, name, job));
	return files;
}

/** ownPlace: a path in one of charter's own places, which are charter's whatever is in them: its folder
 * in the docs, its forms and workflows (charter-*). Other names are the repo's. */
function ownPlace(path: string): boolean {
	const base = basename(path);
	return path.startsWith(`${charterFolder}/`) || path.includes(`/${charterFolder}/`) || base.startsWith("charter-");
}

/** writeFiles writes charter's files into the repo at root: charter's own places as this version has them, with
 * anything else in them removed, a started file only where there is none, and a file at a name the host
 * fixes only where it is charter's. With check it writes nothing. It returns what it wrote (or would),
 * and the repo's own files it kept. */
export function writeFiles(root: string, check: boolean): { changed: string[]; kept: string[] } {
	const files = filesFor(root);
	const changed: string[] = [], kept: string[] = [];
	for (const [name, content] of files) {
		const path = join(root, name);
		if (markOf(content)?.charters === false && existsSync(path)) continue; // started: the repo's once written
		if (existsSync(path)) {
			const have = readFileSync(path, "utf8");
			if (have === content) continue;
			if (!ownPlace(name) && markOf(have)?.charters !== true) {
				kept.push(name);
				continue;
			}
		}
		changed.push(name);
		if (!check) {
			mkdirSync(dirname(path), { recursive: true });
			writeFileSync(path, content);
		}
	}
	const docs = settings(root).docs;
	for (const [dir, match] of [
		[posix.join(docs, charterFolder), () => true],
		[".github/ISSUE_TEMPLATE", (name: string) => name.startsWith("charter-")],
		[".github/workflows", (name: string) => name.startsWith("charter-")],
	] as const) {
		for (const rel of walk(join(root, dir))) {
			const name = posix.join(dir, rel);
			if (files.has(name) || !match(basename(rel))) continue;
			changed.push(`removed ${name}`);
			if (!check) rmSync(join(root, name));
		}
	}
	return { changed: changed.sort(), kept: kept.sort() };
}
