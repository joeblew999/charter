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
	write(dir, "docs/self/README.md", "---\ntitle: Home\nnav_order: 1\npermalink: /\n---\n\n# Our docs\n\n[Rules](rules.md).\n");
	write(dir, ".github/ISSUE_TEMPLATE/bug.yml", "name: Our bug form\ndescription: ours\nbody: []\n");
	spawnSync("git", ["init", "-q", "-b", "main"], { cwd: dir });
	return dir;
}

describe("adopt", () => {
	const dir = repo("adopt");
	const first = charter(dir, "adopt", "--description", "A repo of ours");

	it("writes charter's files, package.json with charter-ts and the scripts, and node_modules/ in .gitignore", () => {
		assert.equal(first.code, 0, first.out);
		for (const file of ["docs/charter/README.md", "docs/charter/rules.md", "docs/charter/writing.md", "docs/self/rules.md", "docs/_config.yml", ".github/workflows/charter-repo.yml", ".github/workflows/charter-ci.yml", ".github/ISSUE_TEMPLATE/charter-plan.yml", "AGENTS.md", "renovate.json"]) {
			assert.ok(existsSync(join(dir, file)), `${file} is written`);
		}
		const pkg = JSON.parse(read(dir, "package.json")) as { description: string; scripts: Record<string, string>; devDependencies: Record<string, string> };
		assert.equal(pkg.description, "A repo of ours");
		assert.deepEqual(pkg.scripts, Object.fromEntries(["adopt", "issues", "check", "docs", "repo", "upstream", "ci"].map(name => [name, `charter-ts ${name}`])));
		assert.equal(pkg.devDependencies["charter-ts"], `https://github.com/joeblew999/charter/releases/download/ts-v${version}/charter-ts-${version}.tgz`);
		assert.match(read(dir, ".gitignore"), /^node_modules\/$/m);
	});

	it("leaves the repo's own files alone, and moves what its CLAUDE.md said into docs/self/rules.md", () => {
		assert.equal(read(dir, "README.md"), "# mine\n");
		assert.match(read(dir, "docs/self/README.md"), /# Our docs/);
		assert.equal(read(dir, ".github/ISSUE_TEMPLATE/bug.yml"), "name: Our bug form\ndescription: ours\nbody: []\n");
		assert.match(read(dir, "docs/self/rules.md"), /Always run the tests\./);
		assert.match(read(dir, "CLAUDE.md"), /Written by `charter-ts adopt`/);
	});

	it("changes nothing when run again, and --check passes", () => {
		const again = charter(dir, "adopt");
		assert.equal(again.code, 0, again.out);
		assert.match(again.out, /ok {7}charter's files: every one is what this version writes/);
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

	it("writes charter's repo job (charter-repo.yml), and a workflow per job of the repo's own ci.config.ts", () => {
		assert.ok(!existsSync(join(dir, "ci.config.ts")), "a repo with no API starts with no CI config of its own");
		const workflow = read(dir, ".github/workflows/charter-repo.yml");
		assert.match(workflow, /^# Written by `charter-ts adopt` \(github\.com\/joeblew999\/charter\): don't edit, it is charter's job/);
		assert.match(workflow, /uses: \.\/\.github\/workflows\/charter-ci\.yml/);
		assert.match(workflow, /permissions:\n {6}contents: read/);
		// A job the repo adds is its own: its workflow is self-<job>.yml.
		write(dir, "ci.config.ts", 'export default { jobs: { lint: { about: "its own", on: ["push"], run: [["echo", "lint"]] } } };\n');
		assert.equal(charter(dir, "adopt").code, 0);
		assert.match(read(dir, ".github/workflows/self-lint.yml"), /^# Written by `charter-ts adopt` from ci\.config\.ts/);
		assert.equal(charter(dir, "adopt", "--check").code, 0);
		// charter's job is charter's: the repo's own cannot take its name.
		write(dir, "ci.config.ts", 'export default { jobs: { repo: { about: "mine", on: ["push"], run: [["echo"]] } } };\n');
		assert.equal(charter(dir, "adopt").code, 1);
		rmSync(join(dir, "ci.config.ts"));
		assert.equal(charter(dir, "adopt").code, 0);
		assert.ok(!existsSync(join(dir, ".github/workflows/self-lint.yml")), "a job that is gone takes its workflow with it");
	});

	it("writes no workflows with \"workflow\": false", () => {
		const bare = repo("no-ci");
		write(bare, "package.json", JSON.stringify({ name: "no-ci", private: true, charter: { workflow: false } }));
		assert.equal(charter(bare, "adopt").code, 0);
		for (const file of [".github/workflows/charter-ci.yml", ".github/workflows/charter-repo.yml"]) assert.ok(!existsSync(join(bare, file)), `${file} is not written`);
	});

	it("replaces a script an older charter-ts wrote under an old name, and keeps the repo's own", () => {
		const old = repo("old-scripts");
		write(old, "package.json", JSON.stringify({ name: "old-scripts", private: true, scripts: { "docs:check": "charter-ts docs:check", "repo:ci": "charter-ts repo:ci", test: "echo ours" } }));
		const adopted = charter(old, "adopt");
		assert.equal(adopted.code, 0, adopted.out);
		const scripts = (JSON.parse(read(old, "package.json")) as { scripts: Record<string, string> }).scripts;
		assert.equal(scripts["docs:check"], undefined);
		assert.equal(scripts["repo:ci"], undefined);
		assert.equal(scripts.test, "echo ours");
		assert.equal(scripts.docs, "charter-ts docs");
	});

	it("an old command's name says what it is called now", () => {
		const old = charter(dir, "docs:check");
		assert.equal(old.code, 1);
		assert.match(old.out, /charter-ts docs:check is charter-ts adopt --check, then docs now/);
	});

	it("docs passes on what adopt wrote, and names a broken link", () => {
		assert.equal(charter(dir, "docs").code, 0);
		write(dir, "docs/self/guide.md", "---\ntitle: Guide\nnav_order: 3\n---\n\n# Guide\n\n[Nowhere](nowhere.md).\n");
		write(dir, "docs/self/README.md", `${read(dir, "docs/self/README.md")}\n[Guide](guide.md).\n`);
		const lint = charter(dir, "docs");
		assert.equal(lint.code, 1);
		assert.match(lint.out, /docs\/self\/guide\.md: link to nowhere\.md: no such file/);
	});
});

describe("new", () => {
	it("copies the notes template under the new name, with both packages pinned to this version, in a repo adopt keeps", () => {
		const into = join(scratch, "made");
		const made = charter(scratch, "new", "demo-api", "--from", "notes", "--subdomain", "example", "--into", into);
		assert.equal(made.code, 0, made.out);
		for (const file of ["charter.config.ts", "cloudflare.config.ts", "worker.ts", "api/contract.ts", "api/handlers.ts", "api/spec.ts", "api/store.ts", "test/live.ts", "test/sdk.ts", "test/access.ts", ".gitignore", ".git", "ci.config.ts", ".github/workflows/self-check.yml", ".github/workflows/charter-repo.yml", ".github/workflows/charter-ci.yml", "docs/charter/README.md", "docs/self/README.md", "AGENTS.md"]) {
			assert.ok(existsSync(join(into, file)), `${file} is there`);
		}
		// It stands alone: its tests import its own contract, and nothing points outside it.
		assert.match(read(into, "test/live.ts"), /from "\.\.\/api\/schemas\.ts"/);
		assert.match(read(into, "tsconfig.node.json"), /"test"/);
		const outside = spawnSync("grep", ["-rl", "-e", "shared/", "-e", "ts/examples-", join(into, "test"), join(into, "charter.config.ts"), join(into, "tsconfig.node.json")], { encoding: "utf8" }).stdout.trim();
		assert.equal(outside, "", `it points outside itself:\n${outside}`);
		const pkg = JSON.parse(read(into, "package.json")) as { name: string; scripts: Record<string, string>; dependencies: Record<string, string>; devDependencies: Record<string, string> };
		assert.equal(pkg.name, "demo-api");
		// A project's scripts, then those every repo has: each the command of its name.
		assert.deepEqual(Object.keys(pkg.scripts), ["dev", "check", "spec", "sdk", "deploy", "live-test", "adopt", "issues", "docs", "repo", "upstream", "ci"]);
		assert.match(read(into, "ci.config.ts"), /check: \{[^}]*run: \[\["charter-ts", "check"\]\]/);
		assert.match(pkg.dependencies["charter-lib"], new RegExp(`/ts-v${version}/charter-lib-${version}\\.tgz$`));
		assert.match(pkg.devDependencies["charter-ts"], new RegExp(`/ts-v${version}/charter-ts-${version}\\.tgz$`));
		assert.match(read(into, "charter.config.ts"), /worker: "demo-api"/);
		assert.match(read(into, "charter.config.ts"), /https:\/\/demo-api\.example\.workers\.dev/);
		const everything = spawnSync("grep", ["-rl", "-e", "charter-notes-ts", "-e", "gedw99", into], { encoding: "utf8" }).stdout.trim();
		assert.equal(everything, "", `the template's own names are left in:\n${everything}`);
	});

	it("--go copies the Go example with its tests, as its own module, on the release's Go library", () => {
		const into = join(scratch, "made-go");
		const made = charter(scratch, "new", "demo-go", "--from", "start", "--go", "--into", into);
		assert.equal(made.code, 0, made.out);
		for (const file of ["main.go", "api/contract.go", "cmd/spec/main.go", "charter.config.ts", "test/live.ts", "test/schemas.ts", "ci.config.ts", ".github/workflows/self-check.yml"]) assert.ok(existsSync(join(into, file)), `${file} is there`);
		const mod = read(into, "go.mod");
		assert.match(mod, /^module demo-go$/m);
		assert.match(mod, new RegExp(`github\\.com/joeblew999/charter/go/lib v${version.replaceAll(".", "\\.")}`));
		assert.doesNotMatch(mod, /^replace /m);
		assert.match(read(into, "test/live.ts"), /from "\.\/schemas\.ts"/);
		assert.match(read(into, "charter.config.ts"), /node\("test\/live\.ts"/);
		const outside = spawnSync("grep", ["-rl", "-e", "ts/examples-", "-e", "charter/go/examples-", "-e", "charter-start-go", into], { encoding: "utf8" }).stdout.trim();
		assert.equal(outside, "", `it points outside itself, or keeps the example's names:\n${outside}`);
	});

	it("refuses a name that is no Worker's name, and a folder that is not empty", () => {
		assert.equal(charter(scratch, "new", "Bad_Name").code, 1);
		assert.equal(charter(scratch, "new", "demo-api", "--into", join(scratch, "made")).code, 1);
	});
});
