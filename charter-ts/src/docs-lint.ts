// charter-ts docs:lint: what is wrong with docs/ that a program can see: front matter, links and their
// anchors, pages nothing links to, scripts and tasks that don't exist, paths that don't exist, release
// versions that go stale. What needs judgement (is the page true, clear, complete) is for a reader:
// npm run docs:review hands that to Claude.
import { spawnSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { charterFolder } from "./kit.ts";
import { output } from "./scripts.ts";
import { repoRoot } from "./settings.ts";

const mdLink = /\]\(([^)\s]+)\)/g;
const mdHeading = /^#{1,6} +(.+?) *$/gm;
const codeSpan = /`([^`\n]+)`/g;
const fenced = /```[\s\S]*?```/g;
const gitTag = /\/v([0-9]|X\.)/; // go/v0.1.0 is a tag, not a path
const pinnedVersion = /(joeblew999\/charter[A-Za-z0-9/_-]*@v[0-9]+\.[0-9]+\.[0-9]+|releases\/(tag|download)\/v[0-9][0-9A-Za-z.-]*)/g;
const repoPath = /^[A-Za-z0-9_.-]+\/[A-Za-z0-9_./*<>{}-]*$/;
const npmRun = /npm run ([a-z][a-z0-9:-]*)/g;
const miseRun = /mise run ([a-z][a-z0-9:-]*)/g;

function pagesIn(dir: string): string[] {
	return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
		const path = join(dir, entry.name);
		if (entry.name.startsWith("_") || entry.name === "node_modules") return [];
		return entry.isDirectory() ? pagesIn(path) : entry.name.endsWith(".md") ? [path] : [];
	});
}

/** The names a page may run: every package.json's scripts and every mise file's tasks git tracks. */
function runnable(root: string): { scripts: Set<string>; tasks: Set<string> } {
	const scripts = new Set<string>(), tasks = new Set<string>();
	const tracked = output("git", ["ls-files", "--", "*package.json", "package.json", "*mise.toml", "mise.toml", "*tasks.toml"], root).split("\n").filter(Boolean);
	for (const file of tracked) {
		const text = readFileSync(join(root, file), "utf8");
		if (file.endsWith("package.json")) {
			for (const name of Object.keys((JSON.parse(text) as { scripts?: object }).scripts ?? {})) scripts.add(name);
		} else {
			// mise.toml: [tasks.<name>]; a tasks.toml it includes: each top-level table is a task.
			const table = file.endsWith("tasks.toml") ? /^\[("?)([A-Za-z0-9:_-]+)\1\]/gm : /^\[tasks\.("?)([A-Za-z0-9:_-]+)\1\]/gm;
			for (const m of text.matchAll(table)) tasks.add(m[2]);
		}
	}
	return { scripts, tasks };
}

/** A heading makes the anchor the way GitHub and Jekyll do: lower case, spaces to hyphens, punctuation dropped. */
function hasAnchor(file: string, anchor: string): boolean {
	const text = readFileSync(file, "utf8");
	return [...text.matchAll(mdHeading)].some(m => m[1].toLowerCase().replace(/[^a-z0-9 _-]/g, "").replaceAll(" ", "-") === anchor);
}

/** ignored: git ignores the path, or it is in a submodule: a real place a fresh checkout doesn't have yet. */
function ignored(root: string, path: string): boolean {
	if (spawnSync("git", ["check-ignore", "-q", path], { cwd: root }).status === 0) return true;
	const modules = spawnSync("git", ["config", "-f", ".gitmodules", "--get-regexp", "\\.path$"], { cwd: root, encoding: "utf8" });
	return modules.status === 0 && modules.stdout.split("\n").some(line => {
		const sub = line.split(" ")[1];
		return sub !== undefined && (path === sub || path.startsWith(`${sub}/`));
	});
}

export function docsLint(): void {
	const root = repoRoot();
	const docs = join(root, "docs");
	if (!existsSync(docs)) {
		console.log("no docs/ here: nothing to check");
		return;
	}
	const pages = pagesIn(docs).sort();
	const { scripts, tasks } = runnable(root);
	const top = new Set(readdirSync(root));
	// Every page must be reachable: linked from at least one other page.
	const linked = new Set<string>();
	for (const page of pages) {
		for (const m of readFileSync(page, "utf8").matchAll(mdLink)) {
			const target = m[1].split("#")[0];
			if (target && !target.includes("://")) linked.add(join(dirname(page), target));
		}
	}
	const problems: string[] = [];
	const say = (page: string, problem: string) => problems.push(`${relative(root, page)}: ${problem}`);
	for (const page of pages) {
		const text = readFileSync(page, "utf8");
		const front = text.split("\n---")[0];
		const rel = relative(docs, page).replaceAll("\\", "/");
		if (!text.startsWith("---\n") || !front.includes("title:") || !front.includes("nav_order:")) say(page, "no front matter with title and nav_order (the sidebar needs them)");
		if (rel === "README.md" && !front.includes("permalink: /")) say(page, "the start page needs `permalink: /` in its front matter, or the site has no home page");
		if (text.includes("{{") || text.includes("{%")) say(page, "two curly braces together, or a curly brace and a percent sign: Jekyll reads those as template code");
		// charter's folder starts a section of its own: the sidebar and AGENTS.md reach it.
		if (rel !== "README.md" && rel !== `${charterFolder}/README.md` && !linked.has(page)) say(page, "no other page links to it: add it to its section's table (or the home page's)");
		// Links and paths are read from the prose: an example inside code is not a link.
		const prose = text.replace(fenced, "").replace(codeSpan, "");
		for (const m of prose.matchAll(mdLink)) {
			const [target, anchor] = m[1].split("#");
			if (target.includes("://") || target.startsWith("mailto:")) continue;
			const file = target ? join(dirname(page), target) : page;
			if (target && !existsSync(file)) {
				say(page, `link to ${m[1]}: no such file`);
				continue;
			}
			if (anchor && file.endsWith(".md") && !hasAnchor(file, anchor)) say(page, `link to ${m[1]}: no heading makes the anchor #${anchor}`);
		}
		// Plans and findings describe what isn't built or what once was; charter's pages are the same in
		// every repo, with example names of their own: only their links are checked.
		if (rel.includes("plans/") || rel === "findings.md" || rel.startsWith(`${charterFolder}/`)) continue;
		for (const m of text.matchAll(pinnedVersion)) say(page, `\`${m[0]}\`: a hard-coded release version goes stale. Use releases/latest, or the placeholder vX.Y.Z`);
		for (const m of text.matchAll(npmRun)) if (!scripts.has(m[1])) say(page, `\`npm run ${m[1]}\`: no such script in a package.json`);
		if (tasks.size > 0) for (const m of text.matchAll(miseRun)) if (!tasks.has(m[1])) say(page, `\`mise run ${m[1]}\`: no such task in a mise file`);
		for (const m of text.replace(fenced, "").matchAll(codeSpan)) {
			const span = m[1].replace(/[:,.]+$/, "");
			if (!repoPath.test(span) || !top.has(span.split("/")[0]) || /[*<>{}]/.test(span) || span.includes("...") || gitTag.test(span)) continue;
			if (!existsSync(join(root, span)) && !ignored(root, span)) say(page, `\`${span}\`: no such path in the repo`);
		}
	}
	if (problems.length === 0) {
		console.log(`docs: ${pages.length} pages, nothing a program can fault`);
		return;
	}
	console.log(problems.join("\n"));
	throw new Error(`docs: ${problems.length} problems in ${pages.length} pages`);
}
