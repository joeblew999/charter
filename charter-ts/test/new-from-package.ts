// What a user gets: a project made by charter-ts new from the packed package, as the release ships it,
// installed, and its own check passing, in TypeScript and in Go. The packages are packed here, and the
// new project installs them, as it would the release's. new copies its example from an archive of this
// checkout's files (CHARTER_SOURCE), as it would from the release's on GitHub; a Go project takes this
// repo's Go library, as it would the release's; and every project takes this checkout's charter from mise, as
// it would the release's (mise use github:joeblew999/charter), which its check runs.
//   node charter-ts/test/new-from-package.ts [start notes go-start go-notes]   (the ci job new; slow: it installs)
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const projects = process.argv.slice(2).length ? process.argv.slice(2) : ["start", "notes", "go-start", "go-notes"];
const scratch = mkdtempSync(join(tmpdir(), "charter-new-"));
const run = (program: string, args: string[], cwd: string, env: Record<string, string> = {}) => {
	console.log(`$ ${program} ${args.join(" ")}   (in ${cwd})`);
	const result = spawnSync(program, args, { cwd, stdio: "inherit", env: { ...process.env, ...env }, shell: process.platform === "win32" });
	if (result.status !== 0) throw new Error(`${program} ${args.join(" ")} failed (${result.status ?? result.signal}) in ${cwd}`);
};

try {
	// The packages, as a release packs them; and the repo, as GitHub's archive of the release's tag has it.
	run("npm", ["pack", "-w", "charter-ts", "-w", "ts", "--pack-destination", scratch], resolve("."));
	const files = spawnSync("git", ["ls-files", "-co", "--exclude-standard"], { encoding: "utf8" }).stdout.split("\n").filter(file => file && existsSync(file));
	writeFileSync(join(scratch, "files.txt"), files.join("\n"));
	const source = join(scratch, "charter.tar.gz");
	run("tar", ["-czf", source, "-T", join(scratch, "files.txt")], resolve("."));
	const tarball = (name: string) => join(scratch, readdirSync(scratch).find(file => file.startsWith(`${name}-`) && file.endsWith(".tgz"))!);
	const unpacked = join(scratch, "charter-ts");
	mkdirSync(unpacked);
	run("tar", ["-xzf", tarball("charter-ts"), "-C", unpacked], scratch);
	// charter, which runs a project's steps: this checkout's, as mise's version local of the release a repo takes
	// (mise use github:joeblew999/charter), as the go-app job links it.
	const bin = join(scratch, "bin");
	run("go", ["build", "-o", join(bin, "charter"), "./cmd/charter"], resolve("."));
	run("mise", ["link", "-f", "github:joeblew999/charter@local", bin], scratch);
	for (const template of projects) {
		// A name that sorts after github.com: the project's own imports move past the Go library's.
		const into = join(scratch, `my-${template}`);
		const go = template.startsWith("go-");
		// new, run from the package: no charter repo around it, so the template is the packed one.
		run("node", [join(unpacked, "package", "bin", "charter-ts.js"), "new", `my-${template}`, "--from", template.replace(/^go-/, ""), ...(go ? ["--go"] : []), "--into", into], scratch, { CHARTER_SOURCE: source });
		// The release's Go library is not tagged yet: this repo's instead.
		if (go) {
			writeFileSync(join(into, "go.mod"), `${readFileSync(join(into, "go.mod"), "utf8")}\nreplace github.com/joeblew999/charter => ${resolve(".")}\n`);
			run("go", ["mod", "tidy"], into);
		}
		// The release's URLs do not exist yet: the packages packed here instead.
		const file = join(into, "package.json");
		const pkg = JSON.parse(readFileSync(file, "utf8")) as { dependencies?: Record<string, string>; devDependencies?: Record<string, string> };
		if (pkg.dependencies?.["charter-lib"]) pkg.dependencies["charter-lib"] = `file:${tarball("charter-lib")}`;
		if (pkg.devDependencies?.["charter-ts"]) pkg.devDependencies["charter-ts"] = `file:${tarball("charter-ts")}`;
		writeFileSync(file, JSON.stringify(pkg, null, "\t") + "\n");
		run("npm", ["install", "--no-audit", "--no-fund"], into);
		const mise = { MISE_TRUSTED_CONFIG_PATHS: into, MISE_YES: "1" };
		run("mise", ["use", "github:joeblew999/charter@local"], into, mise);
		run("mise", ["exec", "--", "npm", "run", "check"], into, mise);
		console.log(`PASS  a project made by charter-ts new --from ${template.replace(/^go-/, "")}${go ? " --go" : ""}, from the packed package, passes its check`);
	}
} finally {
	rmSync(scratch, { recursive: true, force: true });
}
