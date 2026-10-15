// charter-ts repo [--check]: the repo on GitHub in shape from its package.json: the labels (charter's
// .github/charter-labels.tsv), its description, homepage and topics (package.json's description,
// homepage and keywords), and GitHub Pages serving docs/. It writes no file: adopt writes charter's files.
// --check changes nothing and fails on drift.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { parseArgs } from "node:util";
import { charterFile } from "./charter-files.ts";
import { output } from "./run.ts";
import { repoRoot, type Settings, settings } from "./settings.ts";

interface GitHubRepo {
	nameWithOwner: string;
	name: string;
	description: string;
	homepageUrl: string;
	repositoryTopics: { name: string }[] | null;
	defaultBranchRef: { name: string };
}

/** The labels GitHub gives a new repo: those no issue uses and charter does not list go. */
const githubDefaultLabels = ["bug", "documentation", "duplicate", "enhancement", "good first issue", "help wanted", "invalid", "question", "wontfix"];

export function repo(args: string[]): void {
	const { values } = parseArgs({ args, options: { check: { type: "boolean" } } });
	const check = values.check ?? false;
	const root = repoRoot();
	const s = settings(root);
	if (!s.description) throw new Error("package.json has no description: GitHub shows it as the repo's (npm run adopt writes one)");
	const gh = (...a: string[]) => output("gh", a, root);
	const repo = JSON.parse(gh("repo", "view", "--json", "nameWithOwner,name,description,homepageUrl,repositoryTopics,defaultBranchRef")) as GitHubRepo;
	const owner = repo.nameWithOwner.split("/")[0];
	const homepage = s.homepage ?? `https://${owner.toLowerCase()}.github.io/${repo.name}/`;
	const edit = (flag: string, have: string, want: string) => () => {
		if (have === want) return [];
		if (!check) gh("repo", "edit", repo.nameWithOwner, flag, want);
		return [`${JSON.stringify(have)}, want ${JSON.stringify(want)}`];
	};
	const items: [string, () => string[]][] = [
		["labels", () => labels(gh, repo.nameWithOwner, check)],
		["description", edit("--description", repo.description ?? "", s.description)],
		["homepage", edit("--homepage", repo.homepageUrl ?? "", homepage)],
		["topics", () => topics(gh, repo, s, check)],
		["GitHub Pages", () => pages(gh, repo, s, check)],
	];
	let drift = 0;
	for (const [name, sync] of items) {
		const changes = sync();
		if (changes.length === 0) console.log(`ok       ${name}`);
		else if (check) {
			drift++;
			console.log(`differs  ${name}: ${changes.join("; ")}`);
		} else console.log(`changed  ${name}: ${changes.join("; ")}`);
	}
	if (drift > 0) throw new Error(`${drift} items differ from package.json and charter's labels: npm run repo`);
}

type Gh = (...args: string[]) => string;

/** labels makes the repo's labels charter's, and removes GitHub's default ones that charter does not
 * list and nothing uses. Other labels are the repo's own: kept. */
function labels(gh: Gh, repo: string, check: boolean): string[] {
	const want = readFileSync(charterFile(".github/charter-labels.tsv"), "utf8")
		.split("\n")
		.filter(line => line && !line.startsWith("#"))
		.slice(1) // the column names
		.map(line => line.split("\t"))
		.filter(fields => fields.length === 3);
	const have = new Map((JSON.parse(gh("label", "list", "-R", repo, "--limit", "500", "--json", "name,color,description")) as { name: string; color: string; description: string }[]).map(l => [l.name, `${l.color.toLowerCase()}\t${l.description}`]));
	const changes: string[] = [];
	for (const [name, color, description] of want) {
		if (have.get(name) === `${color}\t${description}`) continue;
		changes.push(`set ${name}`);
		if (!check) gh("label", "create", name, "-R", repo, "--color", color, "--description", description, "--force");
	}
	const listed = new Set(want.map(([name]) => name));
	for (const name of githubDefaultLabels) {
		if (!have.has(name) || listed.has(name)) continue;
		const used = ["issue", "pr"].some(kind => !["", "[]"].includes(gh(kind, "list", "-R", repo, "--label", name, "--state", "all", "--limit", "1", "--json", "number").trim()));
		if (used) continue;
		changes.push(`remove ${name}`);
		if (!check) gh("label", "delete", name, "-R", repo, "--yes");
	}
	return changes;
}

/** topics makes the repo's topics exactly package.json's keywords, charter's own among them. */
function topics(gh: Gh, repo: GitHubRepo, s: Settings, check: boolean): string[] {
	const have = (repo.repositoryTopics ?? []).map(t => t.name);
	const add = s.topics.filter(t => !have.includes(t)), remove = have.filter(t => !s.topics.includes(t));
	const changes = [...(add.length ? [`add ${add.join(", ")}`] : []), ...(remove.length ? [`remove ${remove.join(", ")}`] : [])];
	if (changes.length > 0 && !check) {
		gh("repo", "edit", repo.nameWithOwner, ...(add.length ? ["--add-topic", add.join(",")] : []), ...(remove.length ? ["--remove-topic", remove.join(",")] : []));
	}
	return changes;
}

/** pages turns GitHub Pages on for the docs folder on the default branch, built by GitHub itself (no
 * workflow: GitHub renders the markdown). */
function pages(gh: Gh, repo: GitHubRepo, s: Settings, check: boolean): string[] {
	const path = s.docs === "." ? "/" : `/${s.docs}`;
	const branch = repo.defaultBranchRef.name;
	const endpoint = `repos/${repo.nameWithOwner}/pages`;
	const source = ["-f", `source[branch]=${branch}`, "-f", `source[path]=${path}`];
	let now: { build_type: string; source: { branch: string; path: string } };
	try {
		now = JSON.parse(gh("api", endpoint)) as typeof now;
	} catch (error) {
		if (!String(error).includes("404")) throw error;
		if (!check) gh("api", "-X", "POST", endpoint, ...source);
		return [`off, turn on for ${s.docs}/ on ${branch}`];
	}
	if (now.build_type === "legacy" && now.source.branch === branch && now.source.path === path) return [];
	if (!check) gh("api", "-X", "PUT", endpoint, "-f", "build_type=legacy", ...source);
	return [`${now.build_type} from ${now.source.branch} ${now.source.path}, want built by GitHub from ${branch} ${path}`];
}
