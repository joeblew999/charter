// What charter keeps in a repo is one tree, the kit (kit/ in this package), laid out as the repo has
// it: its pages in docs/charter/, the docs site's files GitHub Pages reads at the top of docs/, the
// issue forms, the labels and the workflow in .github/, each named for charter. The same in every repo,
// charter's own included. Charter writes a file only where it may: in its own places always, elsewhere
// where there is none, or the one there carries its mark. What a repo wrote itself is never written over.
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { basename, dirname, join, posix } from "node:path";
import { packageDir } from "./package.ts";
import { isProject, readPackage, settings } from "./settings.ts";
import { steps } from "./steps.ts";

export const kitDir = join(packageDir, "kit");
export const charterFolder = "charter";

/** A file's mark, in its first lines: Written by `charter-ts <command>` ... don't edit (charter's), or
 * Started by `charter-ts <command>` ... yours to edit (the repo's, written once). */
export function markOf(content: string): { charters: boolean; command: string } | undefined {
	const head = content.split("\n").slice(0, 12).join("\n");
	const m = /(Written|Started) by `charter(?:-ts)? ([a-z:-]+)`/.exec(head);
	if (m?.[1] === "Written" && head.includes("don't edit")) return { charters: true, command: m[2] };
	if (m?.[1] === "Started" && head.includes("yours to edit")) return { charters: false, command: m[2] };
	return undefined;
}

/** marked: the file is there and is charter's. */
export const marked = (path: string) => existsSync(path) && markOf(readFileSync(path, "utf8"))?.charters === true;

/** startedBy is a file a command writes once, with the mark that says so. */
export function startedBy(command: string, path: string, content: string): string {
	const mark = `Started by \`charter-ts ${command}\` (github.com/joeblew999/charter): yours to edit.`;
	if (path.endsWith(".md") && content.startsWith("---\n")) return `---\n# ${mark}\n${content.slice(4)}`;
	if (path.endsWith(".md")) return `<!-- ${mark} -->\n\n${content}`;
	return `# ${mark}\n${content}`;
}

function walk(dir: string, prefix = ""): string[] {
	if (!existsSync(dir)) return [];
	return readdirSync(dir, { withFileTypes: true }).flatMap(entry =>
		entry.isDirectory() ? walk(join(dir, entry.name), `${prefix}${entry.name}/`) : [`${prefix}${entry.name}`],
	);
}

const cell = (text: string) => text.replaceAll("|", "\\|").replaceAll("\n", " ");

/** The page of charter-ts's commands, written from them. */
export function usingPage(): string {
	const rows = Object.values(steps).map(s => `| \`${cell(s.usage)}\`${s.anyRepo ? " *" : ""} | ${cell(s.about)} |`);
	return `---
title: Using charter-ts
nav_order: 2
parent: How this repo is kept
---

# Using charter-ts

Written by \`charter-ts adopt\` from charter-ts itself (the same page in every repo that uses it, at the version its package.json installs): don't edit it here, change the commands in \`charter-ts/src/steps.ts\` in [charter](https://github.com/joeblew999/charter).

charter-ts is the tool this repo is kept with: package.json installs it, and its npm scripts run it.

| To | Run |
|---|---|
| See what there is to run | \`npm run\`; in order, with what each does: [Tasks](tasks.md) |
| Run a step | \`npm run <script>\` |
| Run charter-ts | \`npx charter-ts <command>\` |
| Move to another release | \`npm i -D https://github.com/joeblew999/charter/releases/download/ts-vX.Y.Z/charter-ts-X.Y.Z.tgz\`, then \`npm run adopt\` |
| Read more | [charter's docs](https://joeblew999.github.io/charter/); for machines, [llms.txt](https://joeblew999.github.io/charter/llms.txt) |

## The commands

Those marked * work in any repo; the others in a project, a folder with a \`charter.config.ts\`.

| Command | What it does |
|---|---|
${rows.join("\n")}
`;
}

/** The page of the repo's npm scripts, in the order package.json has them: the flow. */
export function tasksPage(root: string): string {
	const scripts = Object.entries(readPackage(root)?.scripts ?? {}).filter(([name]) => !["prepare", "preinstall", "postinstall", "install"].includes(name));
	const rows = scripts.map(([name, command]) => {
		const step = /^charter-ts ([a-z:-]+)$/.exec(command)?.[1];
		return `| \`${name}\` | ${step && steps[step] ? cell(steps[step].about) : `\`${cell(command)}\``} |`;
	});
	return `---
title: Tasks
nav_order: 4
parent: How this repo is kept
---

# Tasks

Written by \`charter-ts adopt\` from the repo's npm scripts (package.json): don't edit, change the scripts, then \`npm run adopt\`.

Run one with \`npm run <script>\`; its arguments go after \`--\`. In the order you run them:

| Script | What it does |
|---|---|
${rows.join("\n")}
`;
}

/** The kit for the repo at root, by path in the repo: every file of the kit as it is, but those the
 * repo does not take (renovate.json; in a project, or with "workflow": false, the CI config to start
 * from; the job runner without a CI config), and the two pages written from charter-ts and from the
 * repo's scripts. */
export function kitFor(root: string): Map<string, string> {
	const s = settings(root);
	const files = new Map<string, string>();
	for (const rel of walk(kitDir)) {
		if (rel === "renovate.json" && !s.renovate) continue;
		// A repo with no API starts with a CI config of its own (unless "workflow": false), and every
		// repo with one gets the job runner its workflows call.
		const startsCI = s.workflow && !isProject(root);
		if (rel === "ci.config.ts" && !startsCI) continue;
		if (rel === ".github/workflows/charter-ci.yml" && !startsCI && !existsSync(join(root, "ci.config.ts"))) continue;
		const to = rel.startsWith("docs/") ? posix.join(s.docs, rel.slice("docs/".length)) : rel;
		files.set(to, readFileSync(join(kitDir, rel), "utf8"));
	}
	files.set(posix.join(s.docs, charterFolder, "using.md"), usingPage());
	files.set(posix.join(s.docs, charterFolder, "tasks.md"), tasksPage(root));
	return files;
}

/** ownPlace: a path in one of charter's own places, which are charter's whatever is in them: its folder
 * in the docs, its forms and workflows (charter-*.yml, charter.yml). Other names are the host's. */
export function ownPlace(path: string): boolean {
	const base = basename(path);
	return path.startsWith(`${charterFolder}/`) || path.includes(`/${charterFolder}/`) || base.startsWith("charter-") || base === "charter.yml";
}

/** writeKit writes the kit into the repo at root: charter's own places as this version has them, with
 * anything else in them removed, a started file only where there is none, and a file at a name the host
 * fixes only where it is charter's. With check it writes nothing. It returns what it wrote (or would),
 * and the repo's own files it kept. */
export function writeKit(root: string, check: boolean): { changed: string[]; kept: string[] } {
	const files = kitFor(root);
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
		[".github/workflows", (name: string) => name.startsWith("charter-") || name === "charter.yml"],
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
