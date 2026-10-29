// Every charter-ts command: how it is called, where it works, what it does, when a developer runs it,
// and the code that runs it (each loads its modules only when it runs). A project's steps are charter's
// (charter.toml, #121): charter-ts's check runs charter's in one, until charter-ts goes; the rest work in
// any repo.
import { existsSync } from "node:fs";
import { join } from "node:path";
import { charterRepo } from "./package.ts";
import { run } from "./run.ts";
import { isProject } from "./settings.ts";

export interface Command {
	/** How it is called, with what it takes. */
	usage: string;
	/** Where it works: in any repo, or in a project (a folder with a charter.toml) too. */
	where: "repo" | "both";
	/** What it does, in one line. */
	about: string;
	/** When a developer runs it, for docs/charter/README.md; none for one a repo has no script of. */
	when?: string;
	run: (args: string[]) => void | Promise<void>;
}

/** charterStep runs a step of a project with charter, its flags as charter writes them (-fresh): in charter's own
 * checkout, or in a project with a go.mod, the tool of its Go module (go tool charter); elsewhere the charter on the
 * PATH, which a repo takes from mise (mise use github:joeblew999/charter). */
function charterStep(step: string[], args: string[]): void {
	const flags = args.map(arg => arg.replace(/^--/, "-"));
	if (charterRepo || existsSync("go.mod")) run("go", ["tool", "charter", ...step, ...flags]);
	else run("charter", [...step, ...flags]);
}

/** The commands, in the order a developer meets them. Each that a repo runs is the npm script of its name
 * (npm run <name>), so there is one way to run it. */
export const commands: Record<string, Command> = {
	issues: {
		usage: "issues [bug|feature|upstream|plan]",
		where: "repo",
		about: "REMOTE, read-only: the open issues, newest first; with a kind (bug, feature, upstream, plan), the body of a new one and the gh command that files it",
		when: "First, whenever you start; with a kind, to file a bug or a plan",
		run: async args => (await import("./issues.ts")).issues(args),
	},
	check: {
		usage: "check [<folder>] [--fresh]",
		where: "both",
		about: "every check that needs no Cloudflare account: in a project (charter.toml), charter check; in a Go module, gofmt, go vet and go test; at a repo's top, the jobs ci.config.ts lists in check (gh signed in, for the repo job)",
		when: "Before every commit",
		run: async args => {
			const folder = args.find(arg => !arg.startsWith("--"));
			if (folder) {
				if (isProject(folder)) process.chdir(folder);
				else if (existsSync(join(folder, "go.mod"))) return (await import("./go.ts")).goCheck(folder);
				else throw new Error(`${folder} is neither a project (charter.toml) nor a Go module (go.mod)`);
			}
			if (isProject(".")) return charterStep(["check"], args.filter(arg => arg.startsWith("--")));
			return (await import("./ci.ts")).ciCheck();
		},
	},
	docs: {
		usage: "docs [--review [--print]]",
		where: "repo",
		about: "what a program can see wrong in docs/ (front matter, links and anchors, pages nothing links to, scripts and paths that don't exist, pinned release versions); --review has Claude read it for the rest",
		when: "After changing a page in docs/",
		run: async args => {
			if (args.includes("--review")) return (await import("./docs.ts")).docsReview(args.filter(arg => arg !== "--review"));
			(await import("./docs.ts")).docsLint();
		},
	},
	ci: {
		usage: "ci [<job> [<project>]] [--remote]",
		where: "repo",
		about: "a job of ci.config.ts here, as GitHub runs it; with no job, every job GitHub runs on a change; --remote starts the job on GitHub instead",
		when: "To run a job as GitHub does, here; --remote to start it there",
		run: async args => {
			const ci = await import("./ci.ts");
			const rest = args.filter(arg => arg !== "--remote");
			return args.includes("--remote") ? ci.ciRemote(rest) : ci.ci(rest);
		},
	},
	upstream: {
		usage: "upstream",
		where: "repo",
		about: "REMOTE, read-only: the upstream issues the code works around (its `Upstream:` tags), and their state",
		when: "Now and then: a CLOSED one means its workaround can go",
		run: async () => (await import("./upstream.ts")).upstream(),
	},
	repo: {
		usage: "repo [--check]",
		where: "repo",
		about: "REMOTE: the repo on GitHub as package.json says: labels, description, homepage, topics, GitHub Pages; --check changes nothing and fails on drift",
		when: "After changing package.json's description, keywords or homepage",
		run: async args => (await import("./repo.ts")).repo(args),
	},
	adopt: {
		usage: "adopt [--check] [--description <text>]",
		where: "repo",
		about: "write charter's files into the repo (never one of the repo's own), the scripts it lacks, and a workflow per job of ci.config.ts; --check writes nothing and fails if one differs",
		when: "After installing another release of charter-ts, and after changing ci.config.ts",
		run: async args => (await import("./adopt.ts")).adopt(args),
	},
	new: {
		usage: "new <name> [--from start|notes] [--go] [--subdomain <workers.dev subdomain>] [--into <dir>]",
		where: "repo",
		about: "a new project, a copy of a tested example under your name, in TypeScript or (--go) in Go, in a repo of its own that adopt keeps",
		run: async args => (await import("./new.ts")).newProject(args),
	},
	version: { usage: "version", where: "repo", about: "the version of charter-ts this is", run: async () => console.log((await import("./package.ts")).version()) },
};

/** What a command was called before, and what it is now: a repo's old script says where it went. A project's
 * steps are charter's now (charter dev, spec, sdk, deploy, live-test). */
export const renamed: Record<string, string> = {
	run: "charter dev -native",
	build: "check (it builds the Worker)",
	bundle: "check (it measures the Worker)",
	lint: "check",
	"spec:check": "check",
	"test:native": "check",
	"test:workerd": "check",
	"migrate:local": "charter dev (it migrates the local database)",
	clients: "charter sdk -prove",
	"sdk:publish": "charter sdk -publish",
	"go:check": "check <folder>",
	"docs:lint": "docs",
	"docs:check": "adopt --check, then docs",
	"docs:review": "docs --review",
	"repo:ci": "ci repo",
	"upstream:status": "upstream",
	issue: "issues <kind>",
	"ci:write": "adopt (it writes the workflows)",
	"ci:remote": "ci <job> --remote",
};

/** What an old command is called now: charter-ts's, or charter's. */
export const renamedTo = (old: string) => (renamed[old]!.startsWith("charter ") ? renamed[old]! : `charter-ts ${renamed[old]}`);
