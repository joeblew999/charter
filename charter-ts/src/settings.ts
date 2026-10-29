// A repo's settings, from its package.json. description, keywords (its topics on GitHub) and homepage
// are npm's own fields; "charter" holds the rest: { docs, renovate, workflow }.
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { output } from "./run.ts";

export interface PackageJson {
	name?: string;
	private?: boolean;
	description?: string;
	keywords?: string[];
	homepage?: string;
	workspaces?: string[];
	scripts?: Record<string, string>;
	dependencies?: Record<string, string>;
	devDependencies?: Record<string, string>;
	charter?: { docs?: string; renovate?: boolean; workflow?: boolean };
	[field: string]: unknown;
}

export interface Settings {
	description: string;
	/** Its topics on GitHub, charter's own among them. */
	topics: string[];
	homepage?: string;
	/** The docs folder: docs, or . (GitHub Pages serves a site from one of those). */
	docs: "docs" | ".";
	/** Write renovate.json. */
	renovate: boolean;
	/** The workflows: charter's job runner and repo job, and one per job of ci.config.ts (which adopt starts in a project). */
	workflow: boolean;
}

/** The topic every repo charter keeps has, so they can be found. */
const charterTopic = "charter";

/** repoRoot is the top of the git repo charter-ts runs in. */
export function repoRoot(): string {
	try {
		return output("git", ["rev-parse", "--show-toplevel"]).trim();
	} catch {
		throw new Error("this runs in a git repo (git init first)");
	}
}

export function readPackage(root: string): PackageJson | undefined {
	const file = join(root, "package.json");
	return existsSync(file) ? (JSON.parse(readFileSync(file, "utf8")) as PackageJson) : undefined;
}

/** settings are the repo's, with their defaults, checked against what GitHub takes. */
export function settings(root: string): Settings {
	const pkg = readPackage(root) ?? {};
	const topics = [...(pkg.keywords ?? [])];
	for (const topic of topics) {
		if (!/^[a-z0-9][a-z0-9-]{0,49}$/.test(topic)) throw new Error(`package.json keywords: ${JSON.stringify(topic)}: a GitHub topic is lower-case letters, digits and hyphens, up to 50`);
	}
	if (!topics.includes(charterTopic)) topics.push(charterTopic);
	if (topics.length > 20) throw new Error("package.json keywords: GitHub takes up to 20 topics, charter's own among them");
	const docs = pkg.charter?.docs ?? "docs";
	if (docs !== "docs" && docs !== ".") throw new Error('package.json charter.docs is "docs" or "." (GitHub Pages serves a site from one of those)');
	return { description: pkg.description ?? "", topics, homepage: pkg.homepage, docs, renovate: pkg.charter?.renovate ?? true, workflow: pkg.charter?.workflow ?? true };
}

/** A project: a folder with a charter.toml, whose steps are charter's own (#121). */
export const isProject = (root: string) => existsSync(join(root, "charter.toml"));
