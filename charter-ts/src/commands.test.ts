// charter-ts's commands, run as a person runs them (node src/cli.ts <command>) in throwaway folders,
// and what they did checked: the files they wrote, the ones they left alone, what a second run does.
//   node --test src/*.test.ts   (npm run check -w charter-ts runs them)
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, describe, it } from "node:test";
import { fileURLToPath } from "node:url";

const cli = fileURLToPath(new URL("./cli.ts", import.meta.url));
const version = (JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8")) as { version: string }).version;
const scratch = mkdtempSync(join(tmpdir(), "charter-ts-test-"));
after(() => rmSync(scratch, { recursive: true, force: true }));

/** charter-ts <args> in dir: its exit code and everything it printed. */
function charter(dir: string, ...args: string[]): { code: number; out: string } {
	const result = spawnSync(process.execPath, [cli, ...args], { cwd: dir, encoding: "utf8", env: { ...process.env, GH_TOKEN: "" } });
	return { code: result.status ?? -1, out: `${result.stdout}${result.stderr}` };
}
const read = (dir: string, file: string) => readFileSync(join(dir, file), "utf8");
function write(dir: string, file: string, content: string): void {
	mkdirSync(join(dir, file, ".."), { recursive: true });
	writeFileSync(join(dir, file), content);
}
/** A git repo with files of its own, as a person has before adopting. */
function repo(name: string): string {
	const dir = join(scratch, name);
	write(dir, "README.md", "# mine\n");
	write(dir, "CLAUDE.md", "# Our notes\n\nAlways run the tests.\n");
	write(dir, "docs/README.md", "---\ntitle: Home\nnav_order: 1\npermalink: /\n---\n\n# Our docs\n\n[Rules](rules.md).\n");
	write(dir, ".github/ISSUE_TEMPLATE/bug.yml", "name: Our bug form\ndescription: ours\nbody: []\n");
	spawnSync("git", ["init", "-q", "-b", "main"], { cwd: dir });
	return dir;
}

describe("adopt", () => {
	const dir = repo("adopt");
	const first = charter(dir, "adopt", "--description", "A repo of ours");

	it("writes the kit, package.json with charter-ts and the scripts, and node_modules/ in .gitignore", () => {
		assert.equal(first.code, 0, first.out);
		for (const file of ["docs/charter/README.md", "docs/charter/rules.md", "docs/charter/tasks.md", "docs/charter/using.md", "docs/_config.yml", "ci.config.ts", ".github/workflows/self-repo.yml", ".github/workflows/charter-ci.yml", ".github/ISSUE_TEMPLATE/charter-plan.yml", "AGENTS.md", "renovate.json"]) {
			assert.ok(existsSync(join(dir, file)), `${file} is written`);
		}
		const pkg = JSON.parse(read(dir, "package.json")) as { description: string; scripts: Record<string, string>; devDependencies: Record<string, string> };
		assert.equal(pkg.description, "A repo of ours");
		assert.equal(pkg.scripts["repo:ci"], "charter-ts repo:ci");
		assert.equal(pkg.devDependencies["charter-ts"], `https://github.com/joeblew999/charter/releases/download/ts-v${version}/charter-ts-${version}.tgz`);
		assert.match(read(dir, ".gitignore"), /^node_modules\/$/m);
	});

	it("leaves the repo's own files alone, and moves what its CLAUDE.md said into docs/rules.md", () => {
		assert.equal(read(dir, "README.md"), "# mine\n");
		assert.match(read(dir, "docs/README.md"), /# Our docs/);
		assert.equal(read(dir, ".github/ISSUE_TEMPLATE/bug.yml"), "name: Our bug form\ndescription: ours\nbody: []\n");
		assert.match(read(dir, "docs/rules.md"), /Always run the tests\./);
		assert.match(read(dir, "CLAUDE.md"), /Written by `charter-ts adopt`/);
	});

	it("changes nothing when run again, and --check passes", () => {
		const again = charter(dir, "adopt");
		assert.equal(again.code, 0, again.out);
		assert.match(again.out, /ok {7}the kit: every file is what this version writes/);
		assert.equal(charter(dir, "adopt", "--check").code, 0);
	});

	it("--check fails on a file of charter's that was changed, and adopt writes it back", () => {
		const page = "docs/charter/rules.md", kept = read(dir, page);
		write(dir, page, `${kept}\nA line someone added.\n`);
		const check = charter(dir, "adopt", "--check");
		assert.equal(check.code, 1);
		assert.match(check.out, /differs {2}docs\/charter\/rules\.md/);
		assert.equal(charter(dir, "adopt").code, 0);
		assert.equal(read(dir, page), kept);
	});

	it("starts a CI config of the repo's own, with the repo job, and writes its workflow from it", () => {
		assert.match(read(dir, "ci.config.ts"), /^\/\/ Started by `charter-ts adopt`.*yours to edit/);
		const workflow = read(dir, ".github/workflows/self-repo.yml");
		assert.match(workflow, /^# Written by `charter-ts ci:write`/);
		assert.match(workflow, /uses: \.\/\.github\/workflows\/charter-ci\.yml/);
		assert.match(workflow, /permissions:\n {6}contents: read/);
		// A job the repo adds is its own: adopt keeps the config, and writes that job's workflow too.
		write(dir, "ci.config.ts", read(dir, "ci.config.ts").replace("jobs: {", 'jobs: {\n\t\tlint: { about: "its own", on: ["push"], run: [["echo", "lint"]] },'));
		assert.equal(charter(dir, "adopt").code, 0);
		assert.match(read(dir, "ci.config.ts"), /lint: \{ about: "its own"/);
		assert.ok(existsSync(join(dir, ".github/workflows/self-lint.yml")), "the repo's own job has its workflow");
		assert.equal(charter(dir, "adopt", "--check").code, 0);
	});

	it("starts no CI config with \"workflow\": false, and then writes no runner", () => {
		const bare = repo("no-ci");
		write(bare, "package.json", JSON.stringify({ name: "no-ci", private: true, charter: { workflow: false } }));
		assert.equal(charter(bare, "adopt").code, 0);
		for (const file of ["ci.config.ts", ".github/workflows/charter-ci.yml", ".github/workflows/self-repo.yml"]) assert.ok(!existsSync(join(bare, file)), `${file} is not written`);
	});

	it("docs:lint passes on what adopt wrote, and names a broken link", () => {
		assert.equal(charter(dir, "docs:lint").code, 0);
		write(dir, "docs/guide.md", "---\ntitle: Guide\nnav_order: 3\n---\n\n# Guide\n\n[Nowhere](nowhere.md).\n");
		write(dir, "docs/README.md", `${read(dir, "docs/README.md")}\n[Guide](guide.md).\n`);
		const lint = charter(dir, "docs:lint");
		assert.equal(lint.code, 1);
		assert.match(lint.out, /docs\/guide\.md: link to nowhere\.md: no such file/);
	});
});

describe("new", () => {
	it("copies the notes template under the new name, with both packages pinned to this version", () => {
		const into = join(scratch, "made");
		const made = charter(scratch, "new", "demo-api", "--from", "notes", "--subdomain", "example", "--into", into);
		assert.equal(made.code, 0, made.out);
		for (const file of ["charter.config.ts", "cloudflare.config.ts", "worker.ts", "api/contract.ts", "api/handlers.ts", "api/spec.ts", "api/store.ts", "test/live.ts", "test/sdk.ts", "test/access.ts", ".gitignore", ".github/workflows/self-check.yml"]) {
			assert.ok(existsSync(join(into, file)), `${file} is there`);
		}
		// It stands alone: its tests import its own contract, and nothing points outside it.
		assert.match(read(into, "test/live.ts"), /from "\.\.\/api\/contract\.ts"/);
		assert.match(read(into, "tsconfig.node.json"), /"test"/);
		const outside = spawnSync("grep", ["-rl", "-e", "shared/", "-e", "ts/examples-", join(into, "test"), join(into, "charter.config.ts"), join(into, "tsconfig.node.json")], { encoding: "utf8" }).stdout.trim();
		assert.equal(outside, "", `it points outside itself:\n${outside}`);
		const pkg = JSON.parse(read(into, "package.json")) as { name: string; dependencies: Record<string, string>; devDependencies: Record<string, string> };
		assert.equal(pkg.name, "demo-api");
		assert.match(pkg.dependencies["charter-lib"], new RegExp(`/ts-v${version}/charter-lib-${version}\\.tgz$`));
		assert.match(pkg.devDependencies["charter-ts"], new RegExp(`/ts-v${version}/charter-ts-${version}\\.tgz$`));
		assert.match(read(into, "charter.config.ts"), /worker: "demo-api"/);
		assert.match(read(into, "charter.config.ts"), /https:\/\/demo-api\.example\.workers\.dev/);
		const everything = spawnSync("grep", ["-rl", "-e", "charter-notes-ts", "-e", "gedw99", into], { encoding: "utf8" }).stdout.trim();
		assert.equal(everything, "", `the template's own names are left in:\n${everything}`);
	});

	it("refuses a name that is no Worker's name, and a folder that is not empty", () => {
		assert.equal(charter(scratch, "new", "Bad_Name").code, 1);
		assert.equal(charter(scratch, "new", "demo-api", "--into", join(scratch, "made")).code, 1);
	});
});
