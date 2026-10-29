// A Go module's check, for one that is no project (charter's own, at the repo's top): gofmt and go vet, for Wasm
// too, then go test. A project's (charter.toml) is charter's: charter check.
import { readdirSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { output, run } from "./run.ts";

/** goCheck is a Go module's check: its packages' files formatted and vetted, then go test; and vetted for Wasm too
 * (GOOS=js GOARCH=wasm), as a Worker is built, the packages a Worker imports: all but the command (cmd/) and its own
 * (internal/), which run on the host. Only the module's own packages (GOWORK=off): the examples beside them are
 * modules of their own in the workspace, each with a check of its own. */
export function goCheck(dir: string): void {
	const env = { GOWORK: "off" };
	const packages = output("go", ["list", "-e", "-f", "{{.Dir}}", "./..."], dir, env).trim().split("\n").filter(Boolean);
	const files = packages.flatMap(pkg => readdirSync(pkg).filter(name => name.endsWith(".go")).map(name => relative(resolve(dir), join(pkg, name))));
	const unformatted = output("gofmt", ["-l", ...files], dir).trim();
	if (unformatted) throw new Error(`not formatted in ${dir} (gofmt -w writes them as they should be):\n${unformatted}`);
	run("go", ["vet", "./..."], env, dir);
	const host = (pkg: string) => /^(cmd|internal)(\/|$)/.test(relative(resolve(dir), pkg).replaceAll("\\", "/"));
	const worker = packages.filter(pkg => !host(pkg)).map(pkg => `./${relative(resolve(dir), pkg).replaceAll("\\", "/")}`);
	run("go", ["vet", ...worker], { ...env, GOOS: "js", GOARCH: "wasm" }, dir);
	run("go", ["test", "./..."], env, dir);
}
