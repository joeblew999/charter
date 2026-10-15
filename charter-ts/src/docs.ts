// charter-ts docs: what is wrong with a repo's docs that a program can see (front matter, links and
// their anchors, pages nothing links to, scripts and paths that don't exist, release versions that go
// stale); with --review, Claude reads them for what needs judgement: is the page true, clear, complete.
import { spawnSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { parseArgs } from "node:util";
import { charterFolder } from "./charter-files.ts";
import { cliPath, packageDir } from "./package.ts";
import { output } from "./run.ts";
import { repoRoot, settings } from "./settings.ts";

const mdLink = /\]\(([^)\s]+)\)/g;
const mdHeading = /^#{1,6} +(.+?) *$/gm;
const codeSpan = /`([^`\n]+)`/g;
const fenced = /```[\s\S]*?```/g;
const gitTag = /\/v([0-9]|X\.)/; // go/lib/v0.1.0 is a tag, not a path
// A release's version, pinned where it goes stale. An install URL (releases/download/ts-vX.Y.Z/...) is
// not: npm run release writes each one to the version it releases.
const pinnedVersion = /(joeblew999\/charter[A-Za-z0-9/_-]*@v[0-9]+\.[0-9]+\.[0-9]+|releases\/tag\/(?:ts-)?v[0-9][0-9A-Za-z.-]*)/g;
const repoPath = /^[A-Za-z0-9_.-]+\/[A-Za-z0-9_./*<>{}-]*$/;
const npmRun = /npm run ([a-z][a-z0-9:-]*)/g;

function pagesIn(dir: string): string[] {
	return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
		const path = join(dir, entry.name);
		if (entry.name.startsWith("_") || entry.name === "node_modules") return [];
		return entry.isDirectory() ? pagesIn(path) : entry.name.endsWith(".md") ? [path] : [];
	});
}

/** The scripts a page may run: those of every package.json git tracks. */
function scriptsOf(root: string): Set<string> {
	const scripts = new Set<string>();
	for (const file of output("git", ["ls-files", "--", "*package.json", "package.json"], root).split("\n").filter(Boolean)) {
		for (const name of Object.keys((JSON.parse(readFileSync(join(root, file), "utf8")) as { scripts?: object }).scripts ?? {})) scripts.add(name);
	}
	return scripts;
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

/** docsLint checks the repo's docs folder (package.json's charter.docs: docs, or the repo's top). */
export function docsLint(): void {
	const root = repoRoot();
	const docs = join(root, settings(root).docs);
	if (!existsSync(join(docs, "self")) && !existsSync(join(docs, charterFolder))) {
		console.log(`no docs in ${relative(root, docs) || "."}: nothing to check`);
		return;
	}
	const pages = pagesIn(docs).sort();
	const scripts = scriptsOf(root);
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
		if (rel === "self/README.md" && !front.includes("permalink: /")) say(page, "the start page needs `permalink: /` in its front matter, or the site has no home page");
		if (text.includes("{{") || text.includes("{%")) say(page, "two curly braces together, or a curly brace and a percent sign: Jekyll reads those as template code");
		// The two folders' first pages, the repo's own and charter's: the sidebar and AGENTS.md reach them.
		if (rel !== "self/README.md" && rel !== `${charterFolder}/README.md` && !linked.has(page)) say(page, "no other page links to it: add it to its section's table (or the home page's)");
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
		// charter's pages are the same in every repo, with example names of their own: only their links are checked.
		if (rel.startsWith(`${charterFolder}/`)) continue;
		for (const m of text.matchAll(pinnedVersion)) say(page, `\`${m[0]}\`: a release version goes stale here. Use releases/latest, or the placeholder vX.Y.Z`);
		for (const m of text.matchAll(npmRun)) if (!scripts.has(m[1])) say(page, `\`npm run ${m[1]}\`: no such script in a package.json`);
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

/** docsReview has Claude (the claude command, Claude Code) bring the docs up to date and into line with
 * docs/charter/writing.md, with what docsLint found in the prompt (review.md, the same for every repo).
 * It may edit files and run the checks, nothing else. --print only shows the prompt. */
export function docsReview(args: string[]): void {
	const { values } = parseArgs({ args, options: { print: { type: "boolean" } } });
	const root = repoRoot();
	const lint = spawnSync(process.execPath, [cliPath, "docs"], { cwd: root, encoding: "utf8" });
	const prompt = readFileSync(join(packageDir, "review.md"), "utf8").replace("__LINT__", `${lint.stdout}${lint.stderr}`.trim());
	if (values.print) {
		console.log(prompt);
		return;
	}
	console.log("Claude is reviewing docs/ (it prints its report when it is done; watch `git status` for its edits)");
	const claude = spawnSync("claude", ["-p", prompt, "--permission-mode", "acceptEdits", "--allowedTools", "Read,Edit,Write,Glob,Grep,Bash(npm run docs),Bash(npm run),Bash(git status:*),Bash(git diff:*),Bash(ls:*)"], { cwd: root, stdio: ["ignore", "inherit", "inherit"] });
	if (claude.error) throw new Error("docs --review needs the claude command (Claude Code) on PATH");
	if (claude.status !== 0) throw new Error(`claude stopped (${claude.status})`);
}
