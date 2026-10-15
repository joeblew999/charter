// What a user gets: a project made by charter-ts new from the packed package, as the release ships it,
// installed, and its own check passing, in TypeScript and in Go. The packages are packed here
// (charter-ts with its templates, and charter-lib), and the new project installs those, as it would the
// release's; a Go one takes this repo's Go library, as it would the release's.
//   node charter-ts/test/new-from-package.ts [start notes go-start go-notes]   (the ci job new; slow: it installs)
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const templates = process.argv.slice(2).length ? process.argv.slice(2) : ["start", "notes", "go-start", "go-notes"];
const scratch = mkdtempSync(join(tmpdir(), "charter-new-"));
const run = (program: string, args: string[], cwd: string) => {
	console.log(`$ ${program} ${args.join(" ")}   (in ${cwd})`);
	const result = spawnSync(program, args, { cwd, stdio: "inherit", shell: process.platform === "win32" });
	if (result.status !== 0) throw new Error(`${program} ${args.join(" ")} failed (${result.status ?? result.signal}) in ${cwd}`);
};

try {
	// The packages, as a release packs them (charter-ts's prepack makes its templates).
	run("npm", ["pack", "-w", "charter-ts", "-w", "ts/lib", "--pack-destination", scratch], resolve("."));
	const tarball = (name: string) => join(scratch, readdirSync(scratch).find(file => file.startsWith(`${name}-`) && file.endsWith(".tgz"))!);
	const unpacked = join(scratch, "charter-ts");
	mkdirSync(unpacked);
	run("tar", ["-xzf", tarball("charter-ts"), "-C", unpacked], scratch);
	for (const template of templates) {
		const into = join(scratch, `demo-${template}`);
		const go = template.startsWith("go-");
		// new, run from the package: no charter repo around it, so the template is the packed one.
		run("node", [join(unpacked, "package", "bin", "charter-ts.js"), "new", `demo-${template}`, "--from", template.replace(/^go-/, ""), ...(go ? ["--go"] : []), "--into", into], scratch);
		// The release's Go library is not tagged yet: this repo's instead.
		if (go) {
			writeFileSync(join(into, "go.mod"), `${readFileSync(join(into, "go.mod"), "utf8")}\nreplace github.com/joeblew999/charter/go/lib => ${resolve("go/lib")}\n`);
			run("go", ["mod", "tidy"], into);
		}
		// The release's URLs do not exist yet: the packages packed here instead.
		const file = join(into, "package.json");
		const pkg = JSON.parse(readFileSync(file, "utf8")) as { dependencies?: Record<string, string>; devDependencies?: Record<string, string> };
		if (pkg.dependencies?.["charter-lib"]) pkg.dependencies["charter-lib"] = `file:${tarball("charter-lib")}`;
		if (pkg.devDependencies?.["charter-ts"]) pkg.devDependencies["charter-ts"] = `file:${tarball("charter-ts")}`;
		writeFileSync(file, JSON.stringify(pkg, null, "\t") + "\n");
		run("npm", ["install", "--no-audit", "--no-fund"], into);
		run("npm", ["run", "check"], into);
		console.log(`PASS  a project made by charter-ts new --from ${template.replace(/^go-/, "")}${go ? " --go" : ""}, from the packed package, passes its check`);
	}
} finally {
	rmSync(scratch, { recursive: true, force: true });
}
