// The repo's CI, said once, in TypeScript (ci.config.ts at the repo's root), and typechecked: the jobs
// and when GitHub runs them. charter-ts adopt writes a workflow per job from it
// (.github/workflows/self-<job>.yml: self- names the repo's own, charter- charter's), which calls
// charter's job runner (charter-ci.yml): mise installs the tools, npm the packages, then
// `npm run ci -- <job>`. That is the command on a machine too, so a job runs the same here as on
// GitHub; `npm run ci` is every job, and `npm run ci -- <job> --remote` starts one on GitHub from here.
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, posix } from "node:path";
import { pathToFileURL } from "node:url";
import { markOf } from "./charter-files.ts";
import { cliPath } from "./package.ts";
import { bin, output, run, seconds } from "./run.ts";
import { readPackage, repoRoot } from "./settings.ts";

/** A command: a program and its arguments. charter-ts is this charter-ts. */
export type Argv = readonly [program: string, ...args: string[]];

export interface Job {
	about: string;
	/** When GitHub runs it: on a push to main, on a pull request, on a version tag (ts-v*), or by hand. */
	on: readonly ("push" | "pull_request" | "tag" | "dispatch")[];
	/** Once at the repo's root, or in every project's folder: a matrix on GitHub, one after another here. */
	each?: "project";
	/** The projects, by default every workspace with a charter.config.ts. */
	projects?: readonly string[];
	run: Argv[];
	/** The repo's GitHub secrets the job needs, as environment variables of the same names. */
	secrets?: readonly string[];
	/** The systems it runs on, by default Linux. */
	os?: readonly ("linux" | "macos" | "windows")[];
	/** What its GitHub token may do (GitHub's permissions, e.g. { issues: "read" }); by default the repo's. */
	permissions?: Readonly<Record<string, "read" | "write" | "none">>;
	timeoutMinutes?: number;
}

export interface CI {
	/** Jobs: `npm run ci -- <name>` here, the workflow self-<name>.yml on GitHub. */
	jobs: Record<string, Job>;
	/** The jobs `npm run check` runs at the repo's top, one after another (in a project, check is the
	 * project's own): by default every job GitHub runs on a push or a pull request. */
	check?: readonly string[];
}

const runners = { linux: "ubuntu-24.04", macos: "macos-15", windows: "windows-2025" } as const;
const written = "Written by `charter-ts adopt` from ci.config.ts: don't edit, change ci.config.ts, then npm run adopt.";
/** A workflow adopt wrote from ci.config.ts. */
const fromCI = (content: string) => content.includes("from ci.config.ts") && markOf(content)?.command === "adopt";

/** charter's own jobs, the same in every repo: their workflows (charter-<job>.yml) are charter's files. */
export const charterJobs: Record<string, Job> = {
	repo: {
		about: "What charter checks in any repo: charter's files and the workflows (adopt --check), the docs, the repo on GitHub as package.json says (repo --check), the upstream issues the code works around",
		on: ["push", "pull_request"],
		run: [["charter-ts", "adopt", "--check"], ["charter-ts", "docs"], ["charter-ts", "repo", "--check"], ["charter-ts", "upstream"]],
		timeoutMinutes: 10,
		// What the checks read with gh, and nothing they could change.
		permissions: { contents: "read", issues: "read", "pull-requests": "read", pages: "read" },
	},
};

/** The repo's own jobs, from ci.config.ts (none without one). */
async function ownJobs(root: string): Promise<CI> {
	const file = join(root, "ci.config.ts");
	if (!existsSync(file)) return { jobs: {} };
	const own = ((await import(pathToFileURL(file).href)) as { default: CI }).default;
	for (const name of Object.keys(own.jobs)) if (charterJobs[name]) throw new Error(`ci.config.ts has a job ${name}: that one is charter's (charter-${name}.yml), so name yours otherwise`);
	return own;
}

/** Every job: charter's, and the repo's own. */
async function load(root: string): Promise<CI> {
	const own = await ownJobs(root);
	return { ...own, jobs: { ...charterJobs, ...own.jobs } };
}

/** A job's workflow file: charter-<job>.yml for charter's, self-<job>.yml for the repo's own. */
const workflowOf = (name: string) => `${charterJobs[name] ? "charter" : "self"}-${name}.yml`;

/** The projects a job runs in: those it names, or every workspace with a charter.config.ts. */
function projectsOf(root: string, job: Job): string[] {
	if (job.projects) return [...job.projects];
	return (readPackage(root)?.workspaces ?? []).filter(dir => existsSync(join(root, dir, "charter.config.ts")));
}

/** The secrets a job names, from the environment, or on GitHub from CHARTER_SECRETS (the repo's
 * secrets, which the shared runner passes): a job gets those its config names and no others. */
function secretsFor(job: Job): Record<string, string> {
	const passed = process.env.CHARTER_SECRETS ? (JSON.parse(process.env.CHARTER_SECRETS) as Record<string, string>) : {};
	return Object.fromEntries((job.secrets ?? []).flatMap(name => {
		const value = process.env[name] ?? passed[name];
		return value ? [[name, value]] : [];
	}));
}

/** exec runs a command in dir, charter-ts as this charter-ts, with the terminal's output. */
function exec(command: Argv, dir: string, env: Record<string, string>): void {
	const [program, ...args] = command;
	console.log(`$ ${command.join(" ")}${dir === "." ? "" : `   (in ${dir})`}`);
	const [file, argv] = program === "charter-ts" ? [process.execPath, [cliPath, ...args]] : [bin(program, dir), args];
	const { CHARTER_SECRETS: _, ...rest } = process.env;
	const start = performance.now();
	const result = spawnSync(file, argv, { cwd: dir, stdio: "inherit", env: { ...rest, ...env }, shell: process.platform === "win32" && program !== "charter-ts" });
	const took = seconds(performance.now() - start);
	if (result.status !== 0) throw new Error(`${command.join(" ")} failed (${result.status ?? result.signal}) after ${took}${dir === "." ? "" : ` in ${dir}`}`);
	console.log(`took ${took}: ${command.join(" ")}${dir === "." ? "" : `   (in ${dir})`}`);
}

/** The jobs GitHub runs on a push or a pull request (not one run by hand). */
const onEveryChange = (config: CI) => Object.entries(config.jobs).filter(([, job]) => job.on.some(on => on === "push" || on === "pull_request")).map(([job]) => job);

/** ci [<job> [<project>]]: a job, in one project or in each; with no job, every job GitHub runs on a
 * push or a pull request, one after another. Each says how long it took. */
export async function ci(args: string[]): Promise<void> {
	const [name, project] = args;
	const config = await load(repoRoot());
	await runJobs(config, name ? [name] : onEveryChange(config), project);
}

/** check at a repo's top: the jobs ci.config.ts lists in check. */
export async function ciCheck(): Promise<void> {
	const config = await load(repoRoot());
	await runJobs(config, [...(config.check ?? onEveryChange(config))]);
}

async function runJobs(config: CI, names: string[], project?: string): Promise<void> {
	const root = repoRoot();
	const times: string[] = [];
	for (const jobName of names) {
		const job = config.jobs[jobName];
		if (!job) throw new Error(`ci.config.ts has no job ${jobName}: ${Object.keys(config.jobs).join(", ")}`);
		const start = performance.now();
		const dirs = job.each === "project" ? (project ? [project] : projectsOf(root, job)) : ["."];
		for (const dir of dirs) {
			console.log(`\n==== ci ${jobName}${dir === "." ? "" : ` ${dir}`}`);
			for (const command of job.run) exec(command, dir === "." ? root : join(root, dir), secretsFor(job));
		}
		times.push(`${jobName} ${seconds(performance.now() - start)}`);
	}
	console.log(`\nci passed: ${times.join(", ")}`);
}

/** The job runner a repo's workflows call: its own copy, which adopt writes from the charter-ts it has
 * (adopt --check fails while it differs), so it moves with that version. */
const runner = "./.github/workflows/charter-ci.yml";

/** A job's workflow: when it runs, and a call of the shared runner per project and system. Every
 * workflow can be started by hand, so from here: npm run ci -- <job> [<project>] --remote. */
export function workflow(root: string, name: string, job: Job): string {
	const os = (job.os ?? ["linux"]).map(system => runners[system]);
	const projects = job.each === "project" ? projectsOf(root, job) : [];
	const byHand = job.on.length === 1 && job.on[0] === "dispatch";
	const one = `${name}${projects.length ? " [<project>]" : ""}`;
	const mark = charterJobs[name] ? `Written by \`charter-ts adopt\` (github.com/joeblew999/charter): don't edit, it is charter's job, the same in every repo.` : written;
	const lines = [`# ${mark}`, `# ${job.about}. Here: npm run ci -- ${one}; on GitHub, from here: npm run ci -- ${one} --remote.`, `name: ${workflowOf(name).replace(/\.yml$/, "")}`, "on:"];
	const push = [...(job.on.includes("push") ? ["    branches: [main]"] : []), ...(job.on.includes("tag") ? ['    tags: ["ts-v*"]'] : [])];
	if (push.length) lines.push("  push:", ...push);
	if (job.on.includes("pull_request")) lines.push("  pull_request:");
	lines.push("  workflow_dispatch:");
	if (projects.length && byHand) lines.push("    inputs:", "      project:", "        description: The project", "        type: choice", "        required: true", `        options: [${projects.join(", ")}]`);
	else if (projects.length) lines.push("    inputs:", "      project:", "        description: One project (none, every project)", "        type: string", '        default: ""');
	lines.push("jobs:", `  ${name}:`);
	const list = JSON.stringify(projects);
	const matrix = [
		...(projects.length ? [`        project: \${{ fromJSON(inputs.project && format('["{0}"]', inputs.project) || '${list}') }}`] : []),
		...(os.length > 1 ? [`        os: [${os.join(", ")}]`] : []),
	];
	if (matrix.length) lines.push("    strategy:", "      fail-fast: false", "      matrix:", ...matrix);
	if (job.permissions) lines.push("    permissions:", ...Object.entries(job.permissions).map(([scope, access]) => `      ${scope}: ${access}`));
	lines.push(`    uses: ${runner}`, "    with:", `      job: ${name}`);
	if (projects.length) lines.push("      project: ${{ matrix.project }}");
	lines.push(`      runs-on: ${os.length > 1 ? "${{ matrix.os }}" : os[0]}`, `      timeout-minutes: ${job.timeoutMinutes ?? 60}`);
	if (job.secrets?.length) lines.push("    secrets: inherit");
	lines.push("");
	return lines.join("\n");
}

/** ci <job> [<project>] --remote: start the job's workflow on GitHub, on this branch (pushed), and say
 * where the run is. It does not wait for it: gh run watch follows it. */
export async function ciRemote(args: string[]): Promise<void> {
	const root = repoRoot();
	const config = await load(root);
	const [name, project] = args;
	if (!name || !config.jobs[name]) throw new Error(`ci --remote needs a job of ci.config.ts: ${Object.keys(config.jobs).join(", ")}`);
	const branch = output("git", ["branch", "--show-current"], root).trim();
	const started = new Date(Date.now() - 5000).toISOString();
	run("gh", ["workflow", "run", workflowOf(name), "--ref", branch, ...(project ? ["-f", `project=${project}`] : [])]);
	for (let tries = 0; tries < 15; tries++) {
		const runs = JSON.parse(output("gh", ["run", "list", "--workflow", workflowOf(name), "--branch", branch, "--event", "workflow_dispatch", "--limit", "1", "--json", "url,createdAt,databaseId"], root)) as { url: string; createdAt: string; databaseId: number }[];
		if (runs[0] && runs[0].createdAt >= started) {
			console.log(`started: ${runs[0].url}\nnot waiting for it (gh run watch ${runs[0].databaseId} follows it)`);
			return;
		}
		await new Promise(resolve => setTimeout(resolve, 1000));
	}
	console.log(`started ${workflowOf(name)} on ${branch}; gh run list --workflow ${workflowOf(name)} shows it`);
}

/** The workflows from ci.config.ts (adopt writes them), and one written before for a job that is gone,
 * removed. With check it writes nothing and fails if one differs. */
export async function writeWorkflows(root: string, check: boolean): Promise<void> {
	const config = await ownJobs(root);
	const files = new Map<string, string>();
	for (const [name, job] of Object.entries(config.jobs)) files.set(posix.join(".github/workflows", `self-${name}.yml`), workflow(root, name, job));
	const differs: string[] = [];
	for (const [file, content] of files) {
		const path = join(root, file);
		if (existsSync(path) && readFileSync(path, "utf8") === content) continue;
		if (existsSync(path) && !fromCI(readFileSync(path, "utf8"))) throw new Error(`${file} is the repo's own (it does not say adopt wrote it from ci.config.ts): move it, or name the job otherwise`);
		differs.push(file);
		if (!check) {
			mkdirSync(join(path, ".."), { recursive: true });
			writeFileSync(path, content);
		}
	}
	const workflows = join(root, ".github", "workflows");
	for (const name of existsSync(workflows) ? readdirSync(workflows) : []) {
		const file = posix.join(".github/workflows", name);
		if (files.has(file) || !name.startsWith("self-") || !fromCI(readFileSync(join(workflows, name), "utf8"))) continue;
		differs.push(`removed ${file}`);
		if (!check) rmSync(join(workflows, name));
	}
	for (const file of differs) console.log(file.startsWith("removed ") ? (check ? `differs  ${file.slice(8)} (it goes)` : file) : `${check ? "differs" : "wrote  "}  ${file}`);
	if (check && differs.length > 0) throw new Error(`${differs.length} workflows differ from what ci.config.ts says: npm run adopt`);
	if (differs.length === 0) console.log("ok       the CI: the workflows are what ci.config.ts says");
}
