// charter-ts adopt: the repo you are in, kept the charter way. It writes what the repo lacks of
// package.json (charter-ts, the scripts every repo runs), then charter's files (charter-files.ts), moves what a repo's
// own CLAUDE.md or AGENTS.md said into docs/self/rules.md, so charter's take their place, and writes the
// workflows from ci.config.ts (ci.ts), which adopt starts.
import { existsSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { basename, join, resolve } from "node:path";
import { parseArgs } from "node:util";
import { writeWorkflows } from "./ci.ts";
import { marked, writeFiles } from "./charter-files.ts";
import { charterRepo, releaseURL, version } from "./package.ts";
import { output } from "./scripts.ts";
import { renamed } from "./commands.ts";
import { isProject, type PackageJson, readPackage, repoRoot, settings } from "./settings.ts";

/** The scripts every repo kept the charter way runs, in the order it runs them: each is the charter-ts
 * command of its name. */
export const repoScripts: Record<string, string> = Object.fromEntries(["adopt", "issues", "check", "docs", "repo", "upstream", "ci"].map(name => [name, `charter-ts ${name}`]));

const say = (did: string, path: string, why = "") => console.log(`${did.padEnd(7)}  ${path}${why}`);

export async function adopt(args: string[]): Promise<void> {
	const { values } = parseArgs({ args, options: { check: { type: "boolean" }, description: { type: "string" } } });
	const root = repoRoot();
	if (values.check) return adoptCheck(root);

	// 1. package.json: what it lacks; what it has is the repo's.
	const github = gitHub(root);
	const have = readPackage(root);
	const pkg: PackageJson = have ?? { name: github.name ?? basename(root), private: true };
	const old = charterToml(root);
	pkg.description ||= values.description ?? old.description ?? github.description ?? pkg.name;
	pkg.keywords ??= old.topics ?? [];
	if (old.homepage) pkg.homepage ??= old.homepage;
	const scripts = (pkg.scripts ??= {});
	// A script an older charter-ts wrote, under a command's old name, goes: its command is called otherwise now.
	for (const [name, command] of Object.entries(scripts)) {
		const old = /^charter-ts ([a-z:-]+)$/.exec(command)?.[1];
		if (old && renamed[old]) {
			delete scripts[name];
			say("removed", "package.json", `: the script ${name} (charter-ts ${old} is charter-ts ${renamed[old]} now)`);
		}
	}
	const theirs = Object.keys(repoScripts).filter(name => scripts[name] !== undefined && scripts[name] !== repoScripts[name]);
	for (const [name, command] of Object.entries(repoScripts)) scripts[name] ??= command;
	// charter's own repo runs charter-ts from its workspace.
	if (!charterRepo || resolve(charterRepo) !== resolve(root)) (pkg.devDependencies ??= {})["charter-ts"] ??= releaseURL(version());
	const file = join(root, "package.json");
	const text = JSON.stringify(pkg, null, have && !readFileSync(file, "utf8").startsWith("{\n\t") ? 2 : "\t") + "\n";
	if (!have || readFileSync(file, "utf8") !== text) {
		writeFileSync(file, text);
		say(have ? "changed" : "wrote", "package.json", have ? ": what it lacked of charter-ts and the scripts" : "");
	} else say("kept", "package.json", ": it has charter-ts and the scripts");
	if (theirs.length > 0) say("kept", "package.json", `: its own ${theirs.join(", ")}, in place of charter's`);
	if (old.found) {
		rmSync(join(root, "charter.toml"));
		say("moved", "charter.toml", ": its description and topics are package.json's description and keywords");
	}
	// npm's folder is never committed.
	const ignore = join(root, ".gitignore");
	const ignored = existsSync(ignore) ? readFileSync(ignore, "utf8") : "";
	if (!/^\/?node_modules\/?$/m.test(ignored)) {
		writeFileSync(ignore, `${ignored}${ignored && !ignored.endsWith("\n") ? "\n" : ""}node_modules/\n`);
		say(ignored ? "changed" : "wrote", ".gitignore", ": node_modules/");
	}

	// 2. charter's files. A repo's own agent files are taken in step 3, so they are not reported kept here.
	const taking = !isProject(root) && settings(root).docs === "docs";
	writeFilesSaying(root, taking ? ["CLAUDE.md", "AGENTS.md"] : []);

	// 3. A repo's own CLAUDE.md or AGENTS.md: what it said moves into docs/self/rules.md, and charter's take
	// their place. A project's are its own.
	if (taking) {
		let took = false;
		for (const name of ["CLAUDE.md", "AGENTS.md"]) {
			const taken = takeAgentFile(root, name);
			if (taken === "moved") say("moved", name, ": what it said is now in docs/self/rules.md");
			if (taken === "removed") say("removed", name, ": it only pointed to AGENTS.md");
			took ||= taken !== undefined;
		}
		if (took) writeFilesSaying(root);
	}

	// 4. The workflows, from ci.config.ts: a job each, calling the job runner adopt wrote.
	await writeWorkflows(root, false);
	console.log("\nNext: npm install, then what docs/charter/README.md says, and when.");
}

async function adoptCheck(root: string): Promise<void> {
	const { changed, kept } = writeFiles(root, true);
	for (const name of kept) say("kept", name, ": the repo's own (it does not carry charter's mark)");
	for (const name of changed) console.error(`differs  ${name.replace(/^removed /, "")}${name.startsWith("removed ") ? " (not charter's: it goes)" : ""}: charter-ts adopt writes it`);
	if (changed.length > 0) throw new Error(`${changed.length} of charter's files differ from what this version writes: npm run adopt`);
	say("ok", "charter's files", ": every one is what this version writes");
	await writeWorkflows(root, true);
}

function writeFilesSaying(root: string, quiet: string[] = []): void {
	const { changed, kept } = writeFiles(root, false);
	for (const name of changed) name.startsWith("removed ") ? console.log(name) : say("wrote", name);
	for (const name of kept) if (!quiet.includes(name)) say("kept", name, ": yours (it does not carry charter's mark), so charter leaves it");
	if (changed.length === 0) say("ok", "charter's files", ": every one is what this version writes");
}

/** The repo on GitHub, if it is there yet. */
function gitHub(root: string): { name?: string; description?: string } {
	try {
		return JSON.parse(output("gh", ["repo", "view", "--json", "name,description"], root)) as { name?: string; description?: string };
	} catch {
		return {};
	}
}

/** What a charter.toml said, which package.json now says. */
function charterToml(root: string): { found: boolean; description?: string; topics?: string[]; homepage?: string } {
	const file = join(root, "charter.toml");
	if (!existsSync(file)) return { found: false };
	const text = readFileSync(file, "utf8");
	const value = (key: string) => new RegExp(`^${key} = (.*)$`, "m").exec(text)?.[1];
	const description = value("description"), topics = value("topics"), homepage = value("homepage");
	return {
		found: true,
		description: description && (JSON.parse(description) as string),
		topics: topics ? (JSON.parse(topics) as string[]) : undefined,
		homepage: homepage && (JSON.parse(homepage) as string),
	};
}

/** takeAgentFile moves what a repo's own agent file says (one without charter's mark, and more than a
 * pointer) to the end of docs/self/rules.md, headings one level down and links made relative to it, and
 * removes the file, so charter's takes its place. */
function takeAgentFile(root: string, name: string): "moved" | "removed" | undefined {
	const path = join(root, name);
	if (!existsSync(path) || marked(path)) return undefined;
	const notes = readFileSync(path, "utf8").trim();
	const said = notes !== "" && notes !== "@AGENTS.md";
	if (said) {
		const moved = notes.replace(/^(#{1,5} )/gm, "#$1").replace(/\]\(((?:[^)#:/][^):]*|\.\/[^):]*))\)/g, "](../../$1)");
		const rules = join(root, "docs", "self", "rules.md");
		const before = existsSync(rules) ? readFileSync(rules, "utf8").trimEnd() : "";
		writeFileSync(rules, `${before}\n\n## What ${name} said before charter-ts adopt\n\nMoved here by \`charter-ts adopt\`, as it was. Yours to edit: keep what is still true, where it belongs.\n\n${moved}\n`);
	}
	rmSync(path);
	return said ? "moved" : "removed";
}
